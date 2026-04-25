package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/ziyangli/fsrs/backend/internal/middleware"
)

func stringPointer(value string) *string {
	return &value
}

func TestDeckHandler_Import_RejectsCardsMissingContent(t *testing.T) {
	h := &DeckHandler{}

	body, err := json.Marshal(DeckExport{
		Name: "Imported Deck",
		Cards: []CardExport{
			{Front: "Question", Back: "Answer"},
			{Front: "   ", Back: "Still invalid"},
		},
	})
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/decks/import", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Import(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestDeckHandler_CreateRejectsBlankTrimmedName(t *testing.T) {
	h := &DeckHandler{}

	body, err := json.Marshal(createDeckRequest{
		Name:        "   ",
		Description: "ignored",
	})
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/decks", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestDeckHandler_CreateRejectsOverlongName(t *testing.T) {
	h := &DeckHandler{}

	body, err := json.Marshal(createDeckRequest{
		Name: strings.Repeat("a", maxDeckNameLength+1),
	})
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/decks", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestDeckHandler_CreateRejectsOverlongDescription(t *testing.T) {
	h := &DeckHandler{}

	body, err := json.Marshal(createDeckRequest{
		Name:        "Deck",
		Description: strings.Repeat("a", maxDeckDescriptionLength+1),
	})
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/decks", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "description must be") {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}

func TestDeckHandler_CreateRejectsOverlongCardTemplates(t *testing.T) {
	h := &DeckHandler{}

	body, err := json.Marshal(createDeckRequest{
		Name:                 "Deck",
		NewCardFrontTemplate: stringPointer(strings.Repeat("a", maxCardContentLength+1)),
	})
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/decks", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "front template must be") {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}

func TestDeckTemplateRequestValues(t *testing.T) {
	explicitEmpty := ""
	explicitValue := "Template"

	if got := valueOrEmpty(nil); got != "" {
		t.Fatalf("valueOrEmpty(nil) = %q, want empty string", got)
	}
	if got := valueOrEmpty(&explicitValue); got != explicitValue {
		t.Fatalf("valueOrEmpty(&explicitValue) = %q, want %q", got, explicitValue)
	}
	if got := valueOrFallback(nil, "Existing"); got != "Existing" {
		t.Fatalf("valueOrFallback(nil, fallback) = %q, want Existing", got)
	}
	if got := valueOrFallback(&explicitEmpty, "Existing"); got != "" {
		t.Fatalf("valueOrFallback(&explicitEmpty, fallback) = %q, want empty string", got)
	}
}

func TestDeckHandler_Import_RejectsInvalidCardLink(t *testing.T) {
	h := &DeckHandler{}

	body, err := json.Marshal(DeckExport{
		Name: "Imported Deck",
		Cards: []CardExport{
			{Front: "Question", Back: "Answer", Link: "javascript:alert(1)"},
		},
	})
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/decks/import", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Import(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestDeckHandler_CreateRejectsUnknownFields(t *testing.T) {
	h := &DeckHandler{}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/decks",
		bytes.NewReader([]byte(`{"name":"Deck","extra":"value"}`)),
	)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "unknown fields") {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}

func TestDeckHandler_ImportRejectsUnknownFields(t *testing.T) {
	h := &DeckHandler{}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/decks/import",
		bytes.NewReader([]byte(`{"name":"Imported","cards":[],"extra":"value"}`)),
	)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Import(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "unknown fields") {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}

func TestDeckHandler_ImportRejectsTooManyCards(t *testing.T) {
	h := &DeckHandler{}

	cards := make([]CardExport, maxImportCardCount+1)
	for i := range cards {
		cards[i] = CardExport{
			Front: "Question",
			Back:  "Answer",
		}
	}

	body, err := json.Marshal(DeckExport{
		Name:  "Imported",
		Cards: cards,
	})
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/decks/import", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Import(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "10000 cards or fewer") {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}

func TestSanitizeFilename(t *testing.T) {
	testCases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "replaces unsafe path-like characters",
			in:   "../Biology:/Deck?",
			want: "Biology_Deck",
		},
		{
			name: "strips hidden-file style prefix",
			in:   ".env",
			want: "env",
		},
		{
			name: "falls back for empty sanitized names",
			in:   "???",
			want: "deck",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeFilename(tc.in); got != tc.want {
				t.Fatalf("sanitizeFilename(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
