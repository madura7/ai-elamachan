package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeStore is a Storer stub for handler tests.
type fakeStore struct {
	cats []Category
	err  error
}

func (f *fakeStore) ListCategories(_ context.Context, _ string) ([]Category, error) {
	return f.cats, f.err
}

func get(t *testing.T, h *Handler, url string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestHandler_Success(t *testing.T) {
	parent := "electronics"
	store := &fakeStore{cats: []Category{
		{Slug: "electronics", Name: "ඉලෙක්ට්‍රොනික්", SortOrder: 1},
		{Slug: "mobile_phones", Name: "ජංගම දුරකථන", ParentSlug: &parent, SortOrder: 2},
	}}
	h := NewHandler(store)

	rr := get(t, h, "/api/v1/categories?lang=si")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var cats []Category
	if err := json.Unmarshal(rr.Body.Bytes(), &cats); err != nil {
		t.Fatalf("response not []Category: %v", err)
	}
	if len(cats) != 2 {
		t.Fatalf("len(cats) = %d, want 2", len(cats))
	}
	if cats[1].ParentSlug == nil || *cats[1].ParentSlug != "electronics" {
		t.Errorf("parent_slug = %v, want electronics", cats[1].ParentSlug)
	}
}

func TestHandler_DefaultLang(t *testing.T) {
	called := ""
	store := &fakeStore{}
	h := &Handler{store: &langCapture{store: store, got: &called}}

	rr := get(t, h, "/api/v1/categories") // no lang= param
	_ = rr

	if called != "en" {
		t.Errorf("default lang = %q, want en", called)
	}
}

// langCapture wraps Storer and records the lang argument.
type langCapture struct {
	store Storer
	got   *string
}

func (l *langCapture) ListCategories(ctx context.Context, lang string) ([]Category, error) {
	*l.got = lang
	return l.store.ListCategories(ctx, lang)
}

func TestHandler_InvalidLang(t *testing.T) {
	h := NewHandler(&fakeStore{})

	for _, lang := range []string{"fr", "zh", "xx", "EN"} {
		rr := get(t, h, "/api/v1/categories?lang="+lang)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("lang=%q: status = %d, want 400", lang, rr.Code)
		}
		var env errEnvelope
		if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
			t.Fatalf("lang=%q: body not errEnvelope: %v", lang, err)
		}
		if env.Error.Code != "invalid_lang" {
			t.Errorf("lang=%q: error.code = %q, want invalid_lang", lang, env.Error.Code)
		}
	}
}

func TestHandler_StoreError(t *testing.T) {
	h := NewHandler(&fakeStore{err: errors.New("db down")})

	rr := get(t, h, "/api/v1/categories?lang=en")

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
	var env errEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("body not errEnvelope: %v", err)
	}
	if env.Error.Code != "internal_error" {
		t.Errorf("error.code = %q, want internal_error", env.Error.Code)
	}
}

func TestHandler_EmptyStoreReturnsArray(t *testing.T) {
	h := NewHandler(&fakeStore{cats: nil})

	rr := get(t, h, "/api/v1/categories?lang=en")

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var cats []Category
	if err := json.Unmarshal(rr.Body.Bytes(), &cats); err != nil {
		t.Fatalf("body not []Category: %v", err)
	}
	if cats == nil {
		t.Error("empty store should return [] not null")
	}
}
