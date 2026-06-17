package listings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/madura7/ai-elamachan/backend/internal/auth"
)

// denyPolicy always denies with a fixed code for testing.
type denyPolicy struct{}

func (denyPolicy) CheckCanPost(_ context.Context, _ string) Decision {
	return Decision{
		Allowed:    false,
		Code:       "posting_limit_exceeded",
		Message:    "daily posting limit reached",
		RetryAfter: 9999999999,
	}
}

func TestAllowAllPolicy(t *testing.T) {
	d := AllowAllPolicy{}.CheckCanPost(context.Background(), "user-1")
	if !d.Allowed {
		t.Fatalf("AllowAllPolicy: expected Allowed=true, got false")
	}
}

func TestCreateListing_Unauthorized_NoToken(t *testing.T) {
	h := &Handler{policy: AllowAllPolicy{}}
	body := `{"category":"electronics","content_language":"en","title":"T","description":"D"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/listings", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.createListing(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestCreateListing_PolicyDenied_Returns403(t *testing.T) {
	h := &Handler{policy: denyPolicy{}}
	body := `{"category":"electronics","content_language":"en","title":"T","description":"D"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/listings", strings.NewReader(body))
	req = auth.TestContext(req, "user-abc")
	w := httptest.NewRecorder()

	h.createListing(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("could not decode 403 body: %v", err)
	}
	if resp["code"] != "posting_limit_exceeded" {
		t.Errorf("expected code=posting_limit_exceeded, got %v", resp["code"])
	}
	if resp["message"] == "" {
		t.Error("expected non-empty message")
	}
	if _, ok := resp["retry_after"]; !ok {
		t.Error("expected retry_after field in 403 body")
	}
}
