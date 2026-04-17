package handler

import (
	"strings"
	"testing"
)

func TestValidateDeckDescription(t *testing.T) {
	tests := []struct {
		name        string
		description string
		wantErr     string
	}{
		{
			name:        "blank description is allowed",
			description: "",
		},
		{
			name:        "overlong description",
			description: strings.Repeat("a", maxDeckDescriptionLength+1),
			wantErr:     "description must be 100000 characters or fewer",
		},
		{
			name:        "valid description",
			description: strings.Repeat("a", maxDeckDescriptionLength),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDeckDescription(tt.description)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateDeckDescription() error = %v, want nil", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("validateDeckDescription() error = nil, want %q", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("validateDeckDescription() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateCardContent(t *testing.T) {
	tests := []struct {
		name    string
		front   string
		back    string
		link    string
		wantErr string
	}{
		{
			name:    "front too long",
			front:   strings.Repeat("a", maxCardContentLength+1),
			back:    "Answer",
			wantErr: "front must be 100000 characters or fewer",
		},
		{
			name:    "back too long",
			front:   "Question",
			back:    strings.Repeat("a", maxCardContentLength+1),
			wantErr: "back must be 100000 characters or fewer",
		},
		{
			name:    "link too long",
			front:   "Question",
			back:    "Answer",
			link:    strings.Repeat("a", maxCardLinkLength+1),
			wantErr: "link must be 8192 characters or fewer",
		},
		{
			name:  "large but valid fields",
			front: strings.Repeat("a", maxCardContentLength),
			back:  strings.Repeat("b", maxCardContentLength),
			link:  strings.Repeat("c", maxCardLinkLength),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCardContent(tt.front, tt.back, tt.link)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateCardContent() error = %v, want nil", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("validateCardContent() error = nil, want %q", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("validateCardContent() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateImportCardCount(t *testing.T) {
	tests := []struct {
		name    string
		count   int
		wantErr string
	}{
		{
			name:  "count below limit",
			count: maxImportCardCount,
		},
		{
			name:    "count above limit",
			count:   maxImportCardCount + 1,
			wantErr: "deck import must contain 10000 cards or fewer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateImportCardCount(tt.count)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateImportCardCount() error = %v, want nil", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("validateImportCardCount() error = nil, want %q", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("validateImportCardCount() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}
