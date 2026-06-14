package auth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	// pgx registers the "pgx" driver name with database/sql.
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
)

// Handler serves POST /api/v1/auth/otp/request and POST /api/v1/auth/otp/verify.
type Handler struct {
	store    *Store
	sessions *Sessions
	sender   Sender
	phoneLim RateLimiter
	ipLim    RateLimiter
	cfg      Config
}

// NewHandlerFromEnv constructs a Handler from environment variables.
// Returns an error if required config is absent or if DB/Redis connections fail.
func NewHandlerFromEnv() (*Handler, error) {
	cfg, err := NewConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return NewHandler(cfg)
}

// NewHandler builds a Handler from an already-parsed Config.
func NewHandler(cfg Config) (*Handler, error) {
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("auth: open db: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(3)

	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("auth: parse redis url %q: %w", cfg.RedisURL, err)
	}
	rdb := redis.NewClient(opts)

	var sender Sender
	switch cfg.SMSMode {
	case "dev", "":
		sender = DevStubSender{}
	default:
		return nil, fmt.Errorf("auth: unknown SMS_MODE %q (real provider requires VER-44)", cfg.SMSMode)
	}

	return &Handler{
		store:    NewStore(db, cfg.MaxOTPAttempts),
		sessions: NewSessions(cfg.JWTSecret, cfg.SessionTTL, rdb),
		sender:   sender,
		phoneLim: NewWindowLimiter(cfg.PhoneRateLimit, cfg.PhoneRateWindow),
		ipLim:    NewWindowLimiter(cfg.IPRateLimit, cfg.IPRateWindow),
		cfg:      cfg,
	}, nil
}

// RegisterRoutes wires the two OTP routes onto mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/otp/request", h.requestOTP)
	mux.HandleFunc("POST /api/v1/auth/otp/verify", h.verifyOTP)
}

// Sessions returns the Sessions manager so other handlers can call Verify.
func (h *Handler) Sessions() *Sessions {
	return h.sessions
}

// --- request OTP ---

type requestOTPBody struct {
	Phone   string `json:"phone"`
	Purpose string `json:"purpose"`
}

func (h *Handler) requestOTP(w http.ResponseWriter, r *http.Request) {
	ip := remoteIP(r)
	if !h.ipLim.Allow("ip:" + ip) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests from this IP")
		return
	}

	var body requestOTPBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "malformed JSON body")
		return
	}

	phone, err := NormalizePhone(body.Phone)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_phone", "phone number is not a valid E.164 number")
		return
	}

	if !h.phoneLim.Allow("phone:" + phone) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many OTP requests for this number; try again later")
		return
	}

	purpose := body.Purpose
	if purpose == "" {
		purpose = "login"
	}
	if purpose != "signup" && purpose != "login" {
		writeError(w, http.StatusBadRequest, "invalid_purpose", "purpose must be signup or login")
		return
	}

	code, err := GenerateOTP()
	if err != nil {
		log.Printf("auth: generate otp: %v", err)
		writeError(w, http.StatusInternalServerError, "server_error", "could not generate OTP")
		return
	}

	codeHash, err := HashOTP(code)
	if err != nil {
		log.Printf("auth: hash otp: %v", err)
		writeError(w, http.StatusInternalServerError, "server_error", "could not hash OTP")
		return
	}

	expiresAt := time.Now().Add(h.cfg.OTPExpiryTTL)
	challenge, err := h.store.CreateChallenge(r.Context(), phone, codeHash, purpose, expiresAt)
	if err != nil {
		log.Printf("auth: create challenge for %s: %v", phone, err)
		writeError(w, http.StatusInternalServerError, "server_error", "could not create OTP challenge")
		return
	}

	if err := h.sender.Send(r.Context(), phone, code); err != nil {
		// Non-fatal for dev stub; log and continue so the challenge_id is returned.
		log.Printf("auth: send OTP to %s: %v", phone, err)
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"challenge_id": challenge.ID,
		"expires_at":   challenge.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

// --- verify OTP ---

type verifyOTPBody struct {
	ChallengeID string `json:"challenge_id"`
	Code        string `json:"code"`
}

func (h *Handler) verifyOTP(w http.ResponseWriter, r *http.Request) {
	ip := remoteIP(r)
	if !h.ipLim.Allow("ip:" + ip) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests from this IP")
		return
	}

	var body verifyOTPBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "malformed JSON body")
		return
	}

	if body.ChallengeID == "" || body.Code == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "challenge_id and code are required")
		return
	}

	challenge, err := h.store.GetChallenge(r.Context(), body.ChallengeID)
	if err != nil {
		switch {
		case errors.Is(err, ErrChallengeNotFound):
			writeError(w, http.StatusUnauthorized, "invalid_challenge", "OTP challenge not found")
		case errors.Is(err, ErrChallengeExpired):
			writeError(w, http.StatusUnauthorized, "challenge_expired", "OTP has expired; request a new one")
		case errors.Is(err, ErrChallengeConsumed):
			writeError(w, http.StatusUnauthorized, "challenge_consumed", "OTP already used")
		case errors.Is(err, ErrMaxAttemptsReached):
			writeError(w, http.StatusTooManyRequests, "rate_limited", "too many failed attempts; request a new OTP")
		default:
			log.Printf("auth: get challenge %s: %v", body.ChallengeID, err)
			writeError(w, http.StatusInternalServerError, "server_error", "failed to retrieve challenge")
		}
		return
	}

	// Increment attempt count BEFORE verifying to prevent brute-force by timing.
	if _, err := h.store.IncrAttempt(r.Context(), challenge.ID); err != nil {
		log.Printf("auth: incr attempt for %s: %v", challenge.ID, err)
	}

	if !VerifyOTP(challenge.CodeHash, body.Code) {
		writeError(w, http.StatusUnauthorized, "invalid_code", "OTP code is incorrect")
		return
	}

	// Single-use: consume the challenge atomically.
	if err := h.store.ConsumeChallenge(r.Context(), challenge.ID); err != nil {
		log.Printf("auth: consume challenge %s: %v", challenge.ID, err)
		writeError(w, http.StatusInternalServerError, "server_error", "failed to consume challenge")
		return
	}

	user, err := h.store.FindOrCreateUser(r.Context(), challenge.PhoneE164, "en")
	if err != nil {
		log.Printf("auth: find/create user for %s: %v", challenge.PhoneE164, err)
		writeError(w, http.StatusInternalServerError, "server_error", "failed to resolve user")
		return
	}

	token, expiresAt, err := h.sessions.Issue(r.Context(), user.ID, user.PhoneE164)
	if err != nil {
		log.Printf("auth: issue session for %s: %v", user.ID, err)
		writeError(w, http.StatusInternalServerError, "server_error", "failed to issue session")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token":      token,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
		"user": map[string]any{
			"id":                 user.ID,
			"phone":              user.PhoneE164,
			"display_name":       user.DisplayName,
			"preferred_language": user.PreferredLanguage,
		},
	})
}

// --- helpers ---

type errEnvelope struct {
	Error errBody `json:"error"`
}

type errBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errEnvelope{Error: errBody{Code: code, Message: message}})
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
