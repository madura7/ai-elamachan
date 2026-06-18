package listings

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/madura7/ai-elamachan/backend/internal/auth"
)

// fakeStore is a no-op BlobStore for handler tests that need a non-nil store
// without touching real object storage.
type fakeStore struct{}

func (fakeStore) PresignPut(_ context.Context, _, _ string, _ time.Duration) (string, error) {
	return "https://example.test/upload", nil
}
func (fakeStore) Head(_ context.Context, _ string) (bool, error) { return true, nil }
func (fakeStore) Delete(_ context.Context, _ string) error       { return nil }
func (fakeStore) PublicURL(key string) string                    { return "https://cdn.test/" + key }

func TestPresignImage_StorageDisabled_Returns503(t *testing.T) {
	h := &Handler{} // store is nil
	body := `{"content_type":"image/jpeg","size_bytes":1024}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/listings/abc/images:presign", strings.NewReader(body))
	req.SetPathValue("id", "abc")
	req = auth.TestContext(req, "user-abc")
	w := httptest.NewRecorder()

	h.presignImage(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestPresignImage_Unauthorized_NoToken(t *testing.T) {
	h := &Handler{store: fakeStore{}}
	body := `{"content_type":"image/jpeg","size_bytes":1024}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/listings/abc/images:presign", strings.NewReader(body))
	req.SetPathValue("id", "abc")
	w := httptest.NewRecorder()

	h.presignImage(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestPresignImage_InvalidContentType_Returns400(t *testing.T) {
	h := &Handler{store: fakeStore{}}
	body := `{"content_type":"image/gif","size_bytes":1024}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/listings/abc/images:presign", strings.NewReader(body))
	req.SetPathValue("id", "abc")
	req = auth.TestContext(req, "user-abc")
	w := httptest.NewRecorder()

	h.presignImage(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for gif, got %d", w.Code)
	}
}

func TestPresignImage_OversizeAndZero_Returns400(t *testing.T) {
	cases := map[string]string{
		"oversize": `{"content_type":"image/png","size_bytes":9000000}`, // > 8MB
		"zero":     `{"content_type":"image/png","size_bytes":0}`,
		"negative": `{"content_type":"image/png","size_bytes":-5}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			h := &Handler{store: fakeStore{}}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/listings/abc/images:presign", strings.NewReader(body))
			req.SetPathValue("id", "abc")
			req = auth.TestContext(req, "user-abc")
			w := httptest.NewRecorder()

			h.presignImage(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("%s: expected 400, got %d", name, w.Code)
			}
		})
	}
}

func TestConfirmImage_StorageDisabled_Returns503(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/listings/abc/images:confirm", strings.NewReader(`{"image_id":"x"}`))
	req.SetPathValue("id", "abc")
	req = auth.TestContext(req, "user-abc")
	w := httptest.NewRecorder()

	h.confirmImage(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestDeleteImage_StorageDisabled_Returns503(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/listings/abc/images/img-1", nil)
	req.SetPathValue("id", "abc")
	req.SetPathValue("imageId", "img-1")
	req = auth.TestContext(req, "user-abc")
	w := httptest.NewRecorder()

	h.deleteImage(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestConfirmImage_Unauthorized_NoToken(t *testing.T) {
	h := &Handler{store: fakeStore{}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/listings/abc/images:confirm", strings.NewReader(`{"image_id":"x"}`))
	req.SetPathValue("id", "abc")
	w := httptest.NewRecorder()

	h.confirmImage(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestNewUUID_FormatAndUniqueness(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id, err := newUUID()
		if err != nil {
			t.Fatalf("newUUID error: %v", err)
		}
		if len(id) != 36 {
			t.Fatalf("expected 36-char uuid, got %q (len %d)", id, len(id))
		}
		if id[14] != '4' {
			t.Errorf("expected version nibble '4' at index 14, got %q in %s", id[14], id)
		}
		if seen[id] {
			t.Fatalf("duplicate uuid generated: %s", id)
		}
		seen[id] = true
	}
}
