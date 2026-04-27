package metrics

import (
	"net/http"
	"strings"
)

func (r *MetricsService) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		auth := req.Header.Get("Authorization")
		token, ok := strings.CutPrefix(auth, "Bearer")

		if !ok || strings.TrimSpace(token) != strings.TrimSpace(r.config.Token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, req)
	})
}
