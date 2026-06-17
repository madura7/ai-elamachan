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

const maxMessageLen = 1000

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
	createH := http.Handler(http.HandlerFunc(h.createInquiry))
	inboxH := http.Handler(http.HandlerFunc(h.sellerInbox))
	if h.bearer != nil {
		createH = h.bearer(createH)
		inboxH = h.bearer(inboxH)
	}
	mux.Handle("POST /api/v1/listings/{listingId}/inquiries", createH)
	mux.Handle("GET /api/v1/inquiries", inboxH)
}

// createInquiry serves POST /api/v1/listings/{listingId}/inquiries.
func (h *Handler) createInquiry(w http.ResponseWriter, r *http.Request) {
	buyerID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		apierr.Write(w, http.StatusUnauthorized, "unauthorized", "bearer token required")
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

	// Fetch the listing to validate it exists, is active, and get the seller id.
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

// persistInquiry inserts the inquiry and (on first contact) the connection event
// in a single transaction. It emits one structured log line + metric when the
// connection event is new.
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

	// North-Star metric: emit exactly once per (buyer, listing) first contact.
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
