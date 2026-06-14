package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const ctxKeyUserID contextKey = "auth_user_id"

// BearerMiddleware validates the Authorization: Bearer <token> header and
// injects the authenticated user ID into the request context.
// Used by downstream handlers (listings, etc.) that require authentication.
func (h *Handler) BearerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "unauthorized", "bearer token required")
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")
		userID, err := h.sessions.Verify(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired session")
			return
		}
		ctx := context.WithValue(r.Context(), ctxKeyUserID, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserIDFromContext retrieves the authenticated user ID injected by BearerMiddleware.
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(ctxKeyUserID).(string)
	return id, ok && id != ""
}
