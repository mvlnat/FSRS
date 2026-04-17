package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONBody_AllowsUnknownFieldsWithoutLimit(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	var dst payload
	req := httptest.NewRequest(
		http.MethodPost,
		"/decode",
		strings.NewReader(`{"name":"deck","extra":"value"}`),
	)
	rec := httptest.NewRecorder()

	if !decodeJSONBody(rec, req, &dst, 0) {
		t.Fatalf("expected permissive JSON decode to succeed, got status %d", rec.Code)
	}
	if dst.Name != "deck" {
		t.Fatalf("decoded name = %q, want %q", dst.Name, "deck")
	}
}

func TestDecodeStrictJSONBody_RejectsUnknownFields(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	var dst payload
	req := httptest.NewRequest(
		http.MethodPost,
		"/decode",
		strings.NewReader(`{"name":"deck","extra":"value"}`),
	)
	rec := httptest.NewRecorder()

	if decodeStrictJSONBody(rec, req, &dst, defaultJSONBodyLimit) {
		t.Fatal("expected strict JSON decode to fail")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "unknown fields") {
		t.Fatalf("unexpected response body: %s", rec.Body.String())
	}
}
