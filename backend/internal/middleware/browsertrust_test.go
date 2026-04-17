package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBrowserTrustMiddleware_AllowsUnsafeRequestFromAllowedOrigin(t *testing.T) {
	middleware := NewBrowserTrustMiddleware([]string{"https://fsrs.ziyang.li"})

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/decks", nil)
	req.Header.Set("Origin", "https://fsrs.ziyang.li")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestBrowserTrustMiddleware_BlocksCrossSiteBrowserRequest(t *testing.T) {
	middleware := NewBrowserTrustMiddleware([]string{"https://fsrs.ziyang.li"})

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodDelete, "/api/decks/1", nil)
	req.Header.Set("Origin", "https://attacker.example")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestBrowserTrustMiddleware_BlocksUnexpectedOrigin(t *testing.T) {
	middleware := NewBrowserTrustMiddleware([]string{"https://fsrs.ziyang.li"})

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPut, "/api/cards/1", nil)
	req.Header.Set("Origin", "https://attacker.example")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestBrowserTrustMiddleware_AllowsNonBrowserClientWithoutOrigin(t *testing.T) {
	middleware := NewBrowserTrustMiddleware([]string{"https://fsrs.ziyang.li"})

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/study/1/review", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusAccepted)
	}
}

func TestBrowserTrustMiddleware_LeavesSafeMethodsAlone(t *testing.T) {
	middleware := NewBrowserTrustMiddleware([]string{"https://fsrs.ziyang.li"})

	handler := middleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/decks", nil)
	req.Header.Set("Origin", "https://attacker.example")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusOK)
	}
}
