package inquiries

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/madura7/ai-elamachan/backend/internal/auth"
)

// ---- createInquiry unit tests (no DB) ----

func TestCreateInquiry_Unauthorized_NoToken(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/listings/abc/inquiries",
		strings.NewReader(`{"message":"hello"}`))
	w := httptest.NewRecorder()
	h.createInquiry(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestCreateInquiry_BlankMessage(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/listings/abc/inquiries",
		strings.NewReader(`{"message":"   "}`))
	req = auth.TestContext(req, "buyer-1")
	w := httptest.NewRecorder()
	h.createInquiry(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
	assertErrorCode(t, w, "message_required")
}

func TestCreateInquiry_MessageTooLong(t *testing.T) {
	h := &Handler{}
	long := strings.Repeat("a", maxMessageLen+1)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/listings/abc/inquiries",
		strings.NewReader(`{"message":"`+long+`"}`))
	req = auth.TestContext(req, "buyer-1")
	w := httptest.NewRecorder()
	h.createInquiry(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
	assertErrorCode(t, w, "message_too_long")
}

// ---- buyerLabel unit tests ----

func TestBuyerLabel_DisplayName(t *testing.T) {
	label := buyerLabel("uid-123", "Priya Silva")
	if label != "Priya Silva" {
		t.Fatalf("expected display name, got %q", label)
	}
}

func TestBuyerLabel_PseudonymWhenNoDisplayName(t *testing.T) {
	label := buyerLabel("uid-abc", "")
	if !strings.HasPrefix(label, "Buyer-") {
		t.Fatalf("expected Buyer-XXXX prefix, got %q", label)
	}
}

func TestBuyerLabel_PseudonymStable(t *testing.T) {
	a := buyerLabel("uid-abc", "")
	b := buyerLabel("uid-abc", "")
	if a != b {
		t.Fatalf("expected stable label, got %q != %q", a, b)
	}
}

func TestBuyerLabel_PseudonymDiffers(t *testing.T) {
	a := buyerLabel("uid-abc", "")
	b := buyerLabel("uid-xyz", "")
	if a == b {
		t.Fatalf("expected different labels for different users, got %q == %q", a, b)
	}
}

// ---- helpers ----

func assertErrorCode(t *testing.T, w *httptest.ResponseRecorder, code string) {
	t.Helper()
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(w.Body).Decode(&envelope); err != nil {
		t.Fatalf("could not decode error body: %v", err)
	}
	if envelope.Error.Code != code {
		t.Fatalf("expected error code %q, got %q", code, envelope.Error.Code)
	}
}
