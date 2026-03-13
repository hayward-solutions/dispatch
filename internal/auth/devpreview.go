package auth

import (
	"context"
	"net/http"
)

// DevPreviewAuth is a middleware that injects a fake user into every request,
// bypassing all session/cookie/DB authentication. This is only used when
// DEV_PREVIEW=true is set and allows LLMs to preview UI components with
// mock data without requiring GitHub OAuth.
func DevPreviewAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := &ContextUser{
			ID:         1,
			Login:      "preview-user",
			Name:       "Preview User",
			AvatarURL:  "https://github.com/ghost.png",
			OAuthToken: "dev-preview-token",
		}
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
