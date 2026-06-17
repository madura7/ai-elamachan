package auth

import (
	"context"
	"net/http"
)

// TestContext returns a copy of r with userID injected as if BearerMiddleware
// had validated a session. For use in tests outside the auth package only.
func TestContext(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), ctxKeyUserID, userID)
	return r.WithContext(ctx)
}
