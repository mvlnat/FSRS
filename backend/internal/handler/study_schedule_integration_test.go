//go:build integration
// +build integration

package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIntegration_ScheduleFallsBackToUTCAndIncludesNewCards(t *testing.T) {
	body := `{"email":"schedule@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	authCookie := rec.Result().Cookies()[0]

	body = `{"name":"Schedule Deck","description":""}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	var deck struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&deck); err != nil {
		t.Fatalf("decode deck: %v", err)
	}

	body = `{"front":"What is due today?","back":"This new card should count on today."}`
	req = httptest.NewRequest(http.MethodPost, "/api/decks/"+deck.ID+"/cards", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create card: got status %d, want %d", rec.Code, http.StatusCreated)
	}

	todayUTC := time.Now().UTC().Format("2006-01-02")
	req = httptest.NewRequest(http.MethodGet, "/api/study/schedule?start="+todayUTC+"&end="+todayUTC+"&timezone=Not/AZone", nil)
	req.AddCookie(authCookie)
	rec = httptest.NewRecorder()
	testRouter.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("get schedule: got status %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var calendar []struct {
		Date  string `json:"date"`
		Total int    `json:"total"`
		Decks []struct {
			DeckID   string `json:"deck_id"`
			DeckName string `json:"deck_name"`
			Count    int    `json:"count"`
		} `json:"decks"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&calendar); err != nil {
		t.Fatalf("decode schedule: %v", err)
	}

	if len(calendar) != 1 {
		t.Fatalf("expected 1 calendar day, got %d", len(calendar))
	}
	if calendar[0].Date != todayUTC {
		t.Fatalf("date = %s, want %s", calendar[0].Date, todayUTC)
	}
	if calendar[0].Total != 1 {
		t.Fatalf("total = %d, want 1", calendar[0].Total)
	}
	if len(calendar[0].Decks) != 1 {
		t.Fatalf("expected 1 deck entry, got %d", len(calendar[0].Decks))
	}
	if calendar[0].Decks[0].DeckName != "Schedule Deck" {
		t.Fatalf("deck name = %s, want Schedule Deck", calendar[0].Decks[0].DeckName)
	}
	if calendar[0].Decks[0].Count != 1 {
		t.Fatalf("deck count = %d, want 1", calendar[0].Decks[0].Count)
	}
}
