package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ziyangli/fsrs/backend/internal/middleware"
)

func TestStudyHandler_ReviewRejectsUnknownFields(t *testing.T) {
	h := &StudyHandler{}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/study/"+uuid.NewString()+"/review",
		bytes.NewReader([]byte(`{"rating":3,"extra":"value"}`)),
	)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("cardId", uuid.NewString())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, uuid.New()))
	rec := httptest.NewRecorder()

	h.Review(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "unknown fields") {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}
