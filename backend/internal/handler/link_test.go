package handler

import "testing"

func TestNormalizeOptionalExternalLink(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "empty link is allowed",
			input: "   ",
			want:  "",
		},
		{
			name:  "https link is allowed",
			input: "https://example.com/docs",
			want:  "https://example.com/docs",
		},
		{
			name:    "credential-bearing link is rejected",
			input:   "https://user:pass@example.com/docs",
			wantErr: true,
		},
		{
			name:    "relative link is rejected",
			input:   "/docs",
			wantErr: true,
		},
		{
			name:    "javascript link is rejected",
			input:   "javascript:alert(1)",
			wantErr: true,
		},
		{
			name:    "mailto link is rejected",
			input:   "mailto:test@example.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeOptionalExternalLink(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("normalizeOptionalExternalLink() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizeOptionalExternalLink() = %q, want %q", got, tt.want)
			}
		})
	}
}
