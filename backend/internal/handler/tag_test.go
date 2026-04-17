package handler

import (
	"strings"
	"testing"
)

func TestValidateTagName(t *testing.T) {
	tests := []struct {
		name    string
		tagName string
		wantErr string
	}{
		{
			name:    "blank tag name",
			tagName: "",
			wantErr: "Tag name is required",
		},
		{
			name:    "overlong tag name",
			tagName: strings.Repeat("a", maxTagNameLength+1),
			wantErr: "Tag name must be 100 characters or fewer",
		},
		{
			name:    "valid tag name",
			tagName: "Biology",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTagName(tt.tagName)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateTagName() error = %v, want nil", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("validateTagName() error = nil, want %q", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("validateTagName() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}
