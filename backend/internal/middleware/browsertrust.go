package middleware

import (
	"net/http"
	"net/url"
	"strings"
)

type BrowserTrustMiddleware struct {
	allowedOrigins map[string]struct{}
}

func NewBrowserTrustMiddleware(allowedOrigins []string) *BrowserTrustMiddleware {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if normalized, ok := normalizeRequestOrigin(origin); ok {
			allowed[normalized] = struct{}{}
		}
	}

	return &BrowserTrustMiddleware{allowedOrigins: allowed}
}

func (m *BrowserTrustMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		if strings.EqualFold(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")), "cross-site") {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
			normalizedOrigin, ok := normalizeRequestOrigin(origin)
			if !ok {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			if _, allowed := m.allowedOrigins[normalizedOrigin]; !allowed {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

func normalizeRequestOrigin(origin string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(origin))
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.User != nil {
		return "", false
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", false
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", false
	}

	return scheme + "://" + strings.ToLower(parsed.Host), true
}
