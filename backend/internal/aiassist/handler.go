package aiassist

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// maxMultipartMemory caps how much of the multipart body is buffered in memory
// before spilling to a temp file. The image cap (maxImageBytes) is enforced
// separately on the decoded part.
const maxMultipartMemory = 1 << 20 // 1 MiB

// allowedImageTypes is the closed set of accepted image media types.
var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
}

// RateLimiter gates requests per caller key. The default in-memory
// implementation (NewWindowLimiter) is fine for a single instance; production
// should back this with Redis so the limit is shared across replicas.
type RateLimiter interface {
	// Allow reports whether the caller identified by key may proceed now.
	Allow(key string) bool
}

// SpendGuard is the workspace spend-cap hook. It is consulted before every
// (billable) model call so a runaway or abusive caller cannot blow the AI
// budget. The default implementation is a simple in-process counter; production
// wires this to the real spend ledger.
type SpendGuard interface {
	// Allow reports whether another model call is within the spend cap.
	Allow() bool
}

// Handler serves POST /api/listings/ai-draft. Construct it with NewHandler (or
// NewHandlerFromEnv, which reads ANTHROPIC_API_KEY from the environment / Secret
// Manager per docs/secrets.md).
type Handler struct {
	client    anthropic.Client
	limiter   RateLimiter
	spend     SpendGuard
	userKeyFn func(*http.Request) string
}

// Options configures a Handler. Any nil field falls back to a sensible default.
type Options struct {
	Limiter   RateLimiter
	Spend     SpendGuard
	UserKeyFn func(*http.Request) string
}

// NewHandler builds a Handler around an already-constructed Anthropic client.
// This is the seam unit tests use to inject a client backed by a fake HTTP
// transport (no live API calls).
func NewHandler(client anthropic.Client, opts Options) *Handler {
	h := &Handler{
		client:    client,
		limiter:   opts.Limiter,
		spend:     opts.Spend,
		userKeyFn: opts.UserKeyFn,
	}
	if h.limiter == nil {
		// Conservative default: 10 drafts/minute/user.
		h.limiter = NewWindowLimiter(10, defaultWindow)
	}
	if h.spend == nil {
		h.spend = allowAllSpend{}
	}
	if h.userKeyFn == nil {
		h.userKeyFn = defaultUserKey
	}
	return h
}

// errEnvelope is the canonical error response shape (ADR 0003):
// { "error": { "code": "...", "message": "..." } }.
type errEnvelope struct {
	Error errBody `json:"error"`
}

type errBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ServeHTTP implements the endpoint. It validates and bounds the request, then
// delegates the single model call to Draft. It NEVER creates or publishes a
// listing — it returns a draft only.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}

	userKey := h.userKeyFn(r)
	if !h.limiter.Allow(userKey) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many draft requests; slow down")
		return
	}
	if !h.spend.Allow() {
		writeError(w, http.StatusServiceUnavailable, "spend_cap_reached", "AI-assist spend cap reached; try again later")
		return
	}

	keywords, imageB64, imageMedia, err := parseDraftRequest(r)
	if err != nil {
		switch {
		case errors.Is(err, ErrKeywordsTooLong):
			writeError(w, http.StatusRequestEntityTooLarge, "keywords_too_large", "keywords exceed 2 KB")
		case errors.Is(err, ErrImageTooLarge):
			writeError(w, http.StatusRequestEntityTooLarge, "image_too_large", "image exceeds 5 MiB")
		case errors.Is(err, errUnsupportedImageType):
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_image_type", "image must be jpeg, png, webp, or gif")
		case errors.Is(err, errEmptyRequest):
			writeError(w, http.StatusBadRequest, "empty_request", "provide keywords and/or a photo")
		default:
			writeError(w, http.StatusBadRequest, "invalid_request", "could not parse the draft request")
		}
		return
	}

	draft, err := Draft(r.Context(), h.client, keywords, imageB64, imageMedia)
	if err != nil {
		switch {
		case errors.Is(err, ErrKeywordsTooLong):
			writeError(w, http.StatusRequestEntityTooLarge, "keywords_too_large", "keywords exceed 2 KB")
		case errors.Is(err, ErrImageTooLarge):
			writeError(w, http.StatusRequestEntityTooLarge, "image_too_large", "image exceeds 5 MiB")
		default:
			// Upstream/model failure. Do not leak provider error detail.
			writeError(w, http.StatusBadGateway, "ai_assist_failed", "could not generate a draft; please try again")
		}
		return
	}

	writeJSON(w, http.StatusOK, draft)
}

var (
	errUnsupportedImageType = errors.New("aiassist: unsupported image type")
	errEmptyRequest         = errors.New("aiassist: empty request")
)

// parseDraftRequest extracts and bounds the keywords + optional image from a
// multipart form. Sizes are enforced before any model call (abuse/cost bound).
func parseDraftRequest(r *http.Request) (keywords, imageB64, imageMedia string, err error) {
	if err = r.ParseMultipartForm(maxMultipartMemory); err != nil {
		return "", "", "", err
	}

	keywords = strings.TrimSpace(r.FormValue("keywords"))
	if len(keywords) > maxKeywordBytes {
		return "", "", "", ErrKeywordsTooLong
	}

	file, header, ferr := r.FormFile("image")
	if ferr == http.ErrMissingFile {
		if keywords == "" {
			return "", "", "", errEmptyRequest
		}
		return keywords, "", "", nil
	}
	if ferr != nil {
		return "", "", "", ferr
	}
	defer file.Close()

	imageMedia = header.Header.Get("Content-Type")
	if !allowedImageTypes[imageMedia] {
		return "", "", "", errUnsupportedImageType
	}

	// Bound the read at the cap + 1 byte so an oversize upload is rejected
	// without buffering the whole thing.
	raw, rerr := io.ReadAll(io.LimitReader(file, maxImageBytes+1))
	if rerr != nil {
		return "", "", "", rerr
	}
	if len(raw) > maxImageBytes {
		return "", "", "", ErrImageTooLarge
	}

	imageB64 = base64.StdEncoding.EncodeToString(raw)
	return keywords, imageB64, imageMedia, nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errEnvelope{Error: errBody{Code: code, Message: message}})
}

// defaultUserKey identifies the caller for rate limiting. Once auth middleware
// (ADR 0002, JWT) lands it should set an authenticated user id header; until
// then we fall back to the remote address so the limit still bites.
func defaultUserKey(r *http.Request) string {
	if uid := r.Header.Get("X-User-ID"); uid != "" {
		return "user:" + uid
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	return "ip:" + host
}

// allowAllSpend is the no-op SpendGuard used when no cap is configured.
type allowAllSpend struct{}

func (allowAllSpend) Allow() bool { return true }
