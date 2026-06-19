package inquiries

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/madura7/ai-elamachan/backend/internal/apierr"
	"github.com/madura7/ai-elamachan/backend/internal/auth"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	maxMessageLen = 1000
	maxReasonLen  = 500
	rateLimitMin  = 5  // messages per minute
	rateLimitHour = 60 // messages per hour
)

// Handler serves inquiry and seller-inbox endpoints.
type Handler struct {
	db     *sql.DB
	bearer func(http.Handler) http.Handler
}

// NewHandlerFromEnv constructs a Handler from DATABASE_URL.
func NewHandlerFromEnv() (*Handler, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, fmt.Errorf("inquiries: DATABASE_URL not set")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("inquiries: open db: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(3)
	return &Handler{db: db}, nil
}

// SetBearer wires in the bearer auth middleware.
func (h *Handler) SetBearer(bearer func(http.Handler) http.Handler) {
	h.bearer = bearer
}

// RegisterRoutes wires inquiry routes onto mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	wrap := func(fn http.HandlerFunc) http.Handler {
		var hh http.Handler = fn
		if h.bearer != nil {
			hh = h.bearer(hh)
		}
		return hh
	}

	mux.Handle("POST /api/v1/listings/{listingId}/inquiries", wrap(h.createInquiry))
	mux.Handle("GET /api/v1/inquiries", wrap(h.sellerInbox))
	mux.Handle("GET /api/v1/inquiries/{inquiryId}", wrap(h.getThread))
	mux.Handle("POST /api/v1/inquiries/{inquiryId}/messages", wrap(h.postMessage))
	mux.Handle("POST /api/v1/inquiries/{inquiryId}/report", wrap(h.reportInquiry))
}

// checkRateLimit returns (retryAfterSeconds, limited).
// Counts messages sent by the user from inquiry_messages in the last minute/hour.
// On DB error or nil DB it is permissive (does not block).
func (h *Handler) checkRateLimit(ctx context.Context, userID string) (retryAfter int, limited bool) {
	if h.db == nil {
		return 0, false
	}
	var countMin int
	if err := h.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM inquiry_messages
		 WHERE sender_user_id = $1 AND created_at > now() - INTERVAL '1 minute'`,
		userID,
	).Scan(&countMin); err != nil {
		return 0, false
	}
	if countMin >= rateLimitMin {
		return 60, true
	}

	var countHour int
	if err := h.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM inquiry_messages
		 WHERE sender_user_id = $1 AND created_at > now() - INTERVAL '1 hour'`,
		userID,
	).Scan(&countHour); err != nil {
		return 0, false
	}
	if countHour >= rateLimitHour {
		return 3600, true
	}
	return 0, false
}

// createInquiry serves POST /api/v1/listings/{listingId}/inquiries.
func (h *Handler) createInquiry(w http.ResponseWriter, r *http.Request) {
	buyerID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		apierr.Write(w, http.StatusUnauthorized, "unauthorized", "bearer token required")
		return
	}

	if retryAfter, limited := h.checkRateLimit(r.Context(), buyerID); limited {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
		apierr.Write(w, http.StatusTooManyRequests, "rate_limited", "too many messages; slow down")
		return
	}

	listingID := r.PathValue("listingId")

	var req InquiryCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.Write(w, http.StatusBadRequest, "invalid_body", "could not parse request body")
		return
	}

	msg := strings.TrimSpace(req.Message)
	if msg == "" {
		apierr.Write(w, http.StatusUnprocessableEntity, "message_required", "message must not be blank")
		return
	}
	if len(msg) > maxMessageLen {
		apierr.Write(w, http.StatusUnprocessableEntity, "message_too_long", "message must be 1000 characters or fewer")
		return
	}

	var sellerID string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT user_id FROM listings WHERE id = $1 AND status = 'active'`,
		listingID,
	).Scan(&sellerID)
	if err == sql.ErrNoRows {
		apierr.Write(w, http.StatusNotFound, "not_found", "listing not found")
		return
	}
	if err != nil {
		log.Printf("inquiries: fetch listing: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not fetch listing")
		return
	}

	if sellerID == buyerID {
		apierr.Write(w, http.StatusUnprocessableEntity, "own_listing", "you cannot inquire about your own listing")
		return
	}

	inq, err := h.persistInquiry(r.Context(), listingID, buyerID, sellerID, msg)
	if err != nil {
		log.Printf("inquiries: persist: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not create inquiry")
		return
	}

	writeJSON(w, http.StatusCreated, inq)
}

// persistInquiry inserts the inquiry, the first buyer message in inquiry_messages,
// and (on first contact) the connection event — all in one transaction.
func (h *Handler) persistInquiry(ctx context.Context, listingID, buyerID, sellerID, message string) (*Inquiry, error) {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var inq Inquiry
	err = tx.QueryRowContext(ctx, `
		INSERT INTO inquiries (listing_id, buyer_user_id, seller_user_id, message)
		VALUES ($1, $2, $3, $4)
		RETURNING id, listing_id, status, created_at
	`, listingID, buyerID, sellerID, message).
		Scan(&inq.ID, &inq.ListingID, &inq.Status, &inq.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert inquiry: %w", err)
	}

	// Seed first message in the thread.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO inquiry_messages (inquiry_id, sender_user_id, sender_role, body, created_at)
		VALUES ($1, $2, 'buyer', $3, $4)
	`, inq.ID, buyerID, message, inq.CreatedAt); err != nil {
		return nil, fmt.Errorf("insert first message: %w", err)
	}

	// Insert connection event; ignore conflict (dedup guarantee).
	res, err := tx.ExecContext(ctx, `
		INSERT INTO connection_events (buyer_user_id, seller_user_id, listing_id, first_inquiry_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (buyer_user_id, listing_id) DO NOTHING
	`, buyerID, sellerID, listingID, inq.ID)
	if err != nil {
		return nil, fmt.Errorf("insert connection_event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	if rows, _ := res.RowsAffected(); rows > 0 {
		log.Printf(`{"event":"connection_event","listing_id":"%s","buyer_user_id":"%s","seller_user_id":"%s","inquiry_id":"%s"}`,
			listingID, buyerID, sellerID, inq.ID)
		connectionEventsTotal.Add(1)
	}

	return &inq, nil
}

// sellerInbox serves GET /api/v1/inquiries — returns inquiries on the caller's
// listings, newest first. Optional ?listing_id= filter.
func (h *Handler) sellerInbox(w http.ResponseWriter, r *http.Request) {
	sellerID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		apierr.Write(w, http.StatusUnauthorized, "unauthorized", "bearer token required")
		return
	}

	filterListingID := r.URL.Query().Get("listing_id")

	query := `
		SELECT
			i.id,
			i.listing_id,
			COALESCE(
				(SELECT title FROM listing_translations WHERE listing_id = i.listing_id AND lang = 'en'),
				(SELECT title FROM listing_translations WHERE listing_id = i.listing_id LIMIT 1),
				''
			) AS listing_title,
			COALESCE(u.display_name, '') AS display_name,
			i.buyer_user_id,
			i.message,
			i.status,
			i.created_at
		FROM inquiries i
		JOIN users u ON u.id = i.buyer_user_id
		WHERE i.seller_user_id = $1
	`
	args := []any{sellerID}
	argIdx := 2

	if filterListingID != "" {
		query += fmt.Sprintf(" AND i.listing_id = $%d", argIdx)
		args = append(args, filterListingID)
		argIdx++
	}
	_ = argIdx
	query += " ORDER BY i.created_at DESC"

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		log.Printf("inquiries: inbox query: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not fetch inquiries")
		return
	}
	defer rows.Close()

	items := make([]SellerInquiry, 0)
	for rows.Next() {
		var si SellerInquiry
		var displayName string
		var buyerUserID string
		if err := rows.Scan(
			&si.ID, &si.ListingID, &si.ListingTitle,
			&displayName, &buyerUserID,
			&si.Message, &si.Status, &si.CreatedAt,
		); err != nil {
			log.Printf("inquiries: scan inbox row: %v", err)
			apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not read inquiry")
			return
		}
		si.BuyerLabel = buyerLabel(buyerUserID, displayName)
		items = append(items, si)
	}
	if err := rows.Err(); err != nil {
		log.Printf("inquiries: inbox rows error: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not read inquiries")
		return
	}

	writeJSON(w, http.StatusOK, items)
}

// getThread serves GET /api/v1/inquiries/{inquiryId}.
// Participant-only. If the caller is the seller and the inquiry is 'new', transitions to 'read'
// (never downgrades 'replied').
func (h *Handler) getThread(w http.ResponseWriter, r *http.Request) {
	callerID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		apierr.Write(w, http.StatusUnauthorized, "unauthorized", "bearer token required")
		return
	}

	inquiryID := r.PathValue("inquiryId")

	// Fetch inquiry header + check participant.
	var thread InquiryThread
	var buyerUserID, sellerUserID string
	var displayName string
	err := h.db.QueryRowContext(r.Context(), `
		SELECT
			i.id,
			i.listing_id,
			COALESCE(
				(SELECT title FROM listing_translations WHERE listing_id = i.listing_id AND lang = 'en'),
				(SELECT title FROM listing_translations WHERE listing_id = i.listing_id LIMIT 1),
				''
			) AS listing_title,
			COALESCE(u.display_name, '') AS display_name,
			i.buyer_user_id,
			i.seller_user_id,
			i.status,
			i.created_at
		FROM inquiries i
		JOIN users u ON u.id = i.buyer_user_id
		WHERE i.id = $1
	`, inquiryID).Scan(
		&thread.ID, &thread.ListingID, &thread.ListingTitle,
		&displayName, &buyerUserID, &sellerUserID,
		&thread.Status, &thread.CreatedAt,
	)
	if err == sql.ErrNoRows {
		apierr.Write(w, http.StatusNotFound, "not_found", "inquiry not found")
		return
	}
	if err != nil {
		log.Printf("inquiries: getThread fetch: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not fetch inquiry")
		return
	}

	if callerID != buyerUserID && callerID != sellerUserID {
		apierr.Write(w, http.StatusForbidden, "forbidden", "not a participant in this inquiry")
		return
	}

	thread.BuyerLabel = buyerLabel(buyerUserID, displayName)

	// Seller opening a 'new' inquiry → transition to 'read' (never downgrade 'replied').
	if callerID == sellerUserID && thread.Status == "new" {
		if _, err := h.db.ExecContext(r.Context(), `
			UPDATE inquiries SET status = 'read', read_at = now()
			WHERE id = $1 AND status = 'new'
		`, inquiryID); err != nil {
			log.Printf("inquiries: mark read: %v", err)
		} else {
			thread.Status = "read"
		}
	}

	// Fetch messages oldest-first.
	msgRows, err := h.db.QueryContext(r.Context(), `
		SELECT id, inquiry_id, sender_role, body, created_at
		FROM inquiry_messages
		WHERE inquiry_id = $1
		ORDER BY created_at ASC
	`, inquiryID)
	if err != nil {
		log.Printf("inquiries: getThread messages: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not fetch messages")
		return
	}
	defer msgRows.Close()

	thread.Messages = make([]InquiryMessage, 0)
	for msgRows.Next() {
		var m InquiryMessage
		if err := msgRows.Scan(&m.ID, &m.InquiryID, &m.SenderRole, &m.Body, &m.CreatedAt); err != nil {
			log.Printf("inquiries: scan message: %v", err)
			apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not read message")
			return
		}
		thread.Messages = append(thread.Messages, m)
	}
	if err := msgRows.Err(); err != nil {
		log.Printf("inquiries: messages rows error: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not read messages")
		return
	}

	writeJSON(w, http.StatusOK, thread)
}

// postMessage serves POST /api/v1/inquiries/{inquiryId}/messages.
// Participant-only. sender_role derived from auth. Seller reply sets status='replied'.
func (h *Handler) postMessage(w http.ResponseWriter, r *http.Request) {
	callerID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		apierr.Write(w, http.StatusUnauthorized, "unauthorized", "bearer token required")
		return
	}

	if retryAfter, limited := h.checkRateLimit(r.Context(), callerID); limited {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
		apierr.Write(w, http.StatusTooManyRequests, "rate_limited", "too many messages; slow down")
		return
	}

	inquiryID := r.PathValue("inquiryId")

	var req MessageCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.Write(w, http.StatusBadRequest, "invalid_body", "could not parse request body")
		return
	}

	body := strings.TrimSpace(req.Body)
	if body == "" {
		apierr.Write(w, http.StatusUnprocessableEntity, "body_required", "body must not be blank")
		return
	}
	if len(body) > maxMessageLen {
		apierr.Write(w, http.StatusUnprocessableEntity, "body_too_long", "body must be 1000 characters or fewer")
		return
	}

	// Fetch inquiry to verify participant and derive sender_role.
	var buyerUserID, sellerUserID string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT buyer_user_id, seller_user_id FROM inquiries WHERE id = $1`,
		inquiryID,
	).Scan(&buyerUserID, &sellerUserID)
	if err == sql.ErrNoRows {
		apierr.Write(w, http.StatusNotFound, "not_found", "inquiry not found")
		return
	}
	if err != nil {
		log.Printf("inquiries: postMessage fetch: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not fetch inquiry")
		return
	}

	var senderRole string
	switch callerID {
	case buyerUserID:
		senderRole = "buyer"
	case sellerUserID:
		senderRole = "seller"
	default:
		apierr.Write(w, http.StatusForbidden, "forbidden", "not a participant in this inquiry")
		return
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Printf("inquiries: postMessage begin tx: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not begin transaction")
		return
	}
	defer tx.Rollback()

	var msg InquiryMessage
	if err := tx.QueryRowContext(r.Context(), `
		INSERT INTO inquiry_messages (inquiry_id, sender_user_id, sender_role, body)
		VALUES ($1, $2, $3, $4)
		RETURNING id, inquiry_id, sender_role, body, created_at
	`, inquiryID, callerID, senderRole, body).
		Scan(&msg.ID, &msg.InquiryID, &msg.SenderRole, &msg.Body, &msg.CreatedAt); err != nil {
		log.Printf("inquiries: postMessage insert: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not save message")
		return
	}

	// Seller reply → set status to 'replied'.
	if senderRole == "seller" {
		if _, err := tx.ExecContext(r.Context(),
			`UPDATE inquiries SET status = 'replied' WHERE id = $1`,
			inquiryID,
		); err != nil {
			log.Printf("inquiries: postMessage update status: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("inquiries: postMessage commit: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not save message")
		return
	}

	writeJSON(w, http.StatusCreated, msg)
}

// reportInquiry serves POST /api/v1/inquiries/{inquiryId}/report.
// Participant-only. Inserts a report row and returns 204.
func (h *Handler) reportInquiry(w http.ResponseWriter, r *http.Request) {
	callerID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		apierr.Write(w, http.StatusUnauthorized, "unauthorized", "bearer token required")
		return
	}

	if retryAfter, limited := h.checkRateLimit(r.Context(), callerID); limited {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
		apierr.Write(w, http.StatusTooManyRequests, "rate_limited", "too many requests; slow down")
		return
	}

	inquiryID := r.PathValue("inquiryId")

	var req InquiryReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierr.Write(w, http.StatusBadRequest, "invalid_body", "could not parse request body")
		return
	}

	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		apierr.Write(w, http.StatusUnprocessableEntity, "reason_required", "reason must not be blank")
		return
	}
	if len(reason) > maxReasonLen {
		apierr.Write(w, http.StatusUnprocessableEntity, "reason_too_long", "reason must be 500 characters or fewer")
		return
	}

	// Verify participant.
	var buyerUserID, sellerUserID string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT buyer_user_id, seller_user_id FROM inquiries WHERE id = $1`,
		inquiryID,
	).Scan(&buyerUserID, &sellerUserID)
	if err == sql.ErrNoRows {
		apierr.Write(w, http.StatusNotFound, "not_found", "inquiry not found")
		return
	}
	if err != nil {
		log.Printf("inquiries: reportInquiry fetch: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not fetch inquiry")
		return
	}

	if callerID != buyerUserID && callerID != sellerUserID {
		apierr.Write(w, http.StatusForbidden, "forbidden", "not a participant in this inquiry")
		return
	}

	if _, err := h.db.ExecContext(r.Context(), `
		INSERT INTO inquiry_reports (inquiry_id, reporter_user_id, reason)
		VALUES ($1, $2, $3)
	`, inquiryID, callerID, reason); err != nil {
		log.Printf("inquiries: reportInquiry insert: %v", err)
		apierr.Write(w, http.StatusInternalServerError, "internal_error", "could not save report")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// buyerLabel returns the buyer's display name if non-empty, else a stable
// pseudonymous label derived from a SHA-256 hash of the buyer_user_id.
// Phone and email are never included.
func buyerLabel(buyerUserID, displayName string) string {
	if strings.TrimSpace(displayName) != "" {
		return displayName
	}
	h := sha256.Sum256([]byte(buyerUserID))
	return fmt.Sprintf("Buyer-%X", h[:2])
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
