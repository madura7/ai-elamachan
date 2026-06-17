// Package inquiries implements buyer→seller inquiry persistence and the seller
// inbox (VER-297, VER-295 M1).
package inquiries

import "time"

// Inquiry is the response body for POST /api/v1/listings/{listingId}/inquiries.
// Intentionally omits seller PII (phone, email, display_name).
type Inquiry struct {
	ID        string    `json:"id"`
	ListingID string    `json:"listing_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// InquiryCreateRequest is the JSON body for the create inquiry endpoint.
type InquiryCreateRequest struct {
	Message string `json:"message"`
}

// SellerInquiry is one row in the seller inbox response.
// buyer_label is a display name if the user has one, else a stable pseudonym
// derived from a hash of buyer_user_id. Phone/email are never included.
type SellerInquiry struct {
	ID           string    `json:"id"`
	ListingID    string    `json:"listing_id"`
	ListingTitle string    `json:"listing_title"`
	BuyerLabel   string    `json:"buyer_label"`
	Message      string    `json:"message"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}
