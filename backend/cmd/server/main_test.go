package main

import (
	"net/http"
	"testing"
)

func TestParseAllowedOrigins_NormalizesAndDeduplicates(t *testing.T) {
	origins, err := parseAllowedOrigins(" https://FSRS.ZIYANG.LI/ , http://localhost:5173,https://fsrs.ziyang.li ")
	if err != nil {
		t.Fatalf("parseAllowedOrigins: %v", err)
	}

	if len(origins) != 2 {
		t.Fatalf("got %d origins, want 2", len(origins))
	}
	if origins[0] != "https://fsrs.ziyang.li" {
		t.Fatalf("origin[0] = %q, want %q", origins[0], "https://fsrs.ziyang.li")
	}
	if origins[1] != "http://localhost:5173" {
		t.Fatalf("origin[1] = %q, want %q", origins[1], "http://localhost:5173")
	}
}

func TestParseAllowedOrigins_RejectsWildcardOrigins(t *testing.T) {
	if _, err := parseAllowedOrigins("https://*.example.com"); err == nil {
		t.Fatal("expected wildcard origin to be rejected")
	}
}

func TestParseAllowedOrigins_RejectsOriginsWithPaths(t *testing.T) {
	if _, err := parseAllowedOrigins("https://fsrs.ziyang.li/app"); err == nil {
		t.Fatal("expected origin with path to be rejected")
	}
}

func TestValidateJWTSecret(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		jwtSecret   string
		wantErr     bool
	}{
		{
			name:        "development allows short secrets",
			environment: "development",
			jwtSecret:   "short",
		},
		{
			name:        "production rejects development default",
			environment: "production",
			jwtSecret:   defaultDevelopmentJWTSecret,
			wantErr:     true,
		},
		{
			name:        "production rejects short secret",
			environment: "production",
			jwtSecret:   "too-short",
			wantErr:     true,
		},
		{
			name:        "production rejects change placeholder",
			environment: "production",
			jwtSecret:   "CHANGE_THIS_TO_RANDOM_32_CHAR_STRING",
			wantErr:     true,
		},
		{
			name:        "production rejects replacement placeholder",
			environment: "production",
			jwtSecret:   "replace-with-a-random-32-byte-production-secret",
			wantErr:     true,
		},
		{
			name:        "production accepts strong secret",
			environment: "production",
			jwtSecret:   "0123456789abcdef0123456789abcdef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJWTSecret(tt.environment, tt.jwtSecret)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestNewHTTPServerSetsDefensiveTimeouts(t *testing.T) {
	server := newHTTPServer(":8080", http.NewServeMux())

	if server.ReadHeaderTimeout != readHeaderTimeout {
		t.Fatalf("ReadHeaderTimeout = %s, want %s", server.ReadHeaderTimeout, readHeaderTimeout)
	}
	if server.ReadTimeout != readTimeout {
		t.Fatalf("ReadTimeout = %s, want %s", server.ReadTimeout, readTimeout)
	}
	if server.WriteTimeout != writeTimeout {
		t.Fatalf("WriteTimeout = %s, want %s", server.WriteTimeout, writeTimeout)
	}
	if server.IdleTimeout != idleTimeout {
		t.Fatalf("IdleTimeout = %s, want %s", server.IdleTimeout, idleTimeout)
	}
}
