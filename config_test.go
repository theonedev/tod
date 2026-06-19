package main

import "testing"

func TestNormalizeServerURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "host with port",
			input: "http://server:6610",
			want:  "http://server:6610",
		},
		{
			name:  "trailing slash",
			input: "http://server:6610/",
			want:  "http://server:6610",
		},
		{
			name:  "https host",
			input: "https://onedev.example.com",
			want:  "https://onedev.example.com",
		},
		{
			name:    "path rejected",
			input:   "http://server:6610/test",
			wantErr: true,
		},
		{
			name:    "nested path rejected",
			input:   "https://onedev.example.com/foo/bar",
			wantErr: true,
		},
		{
			name:    "missing scheme",
			input:   "server:6610",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeServerURL(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
