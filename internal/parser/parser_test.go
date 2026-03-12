package parser

import (
	"strings"
	"testing"
)

func TestCurlToXhArgs(t *testing.T) {
	tests := []struct {
		name     string
		curl     string
		wantArgs string
		wantBody string
	}{
		{
			name:     "simple GET",
			curl:     "curl https://api.github.com/users/jpastorm",
			wantArgs: "https://api.github.com/users/jpastorm",
			wantBody: "",
		},
		{
			name:     "POST with JSON",
			curl:     `curl -X POST https://httpbin.org/post -H "Content-Type: application/json" -d '{"name":"zombie"}'`,
			wantArgs: "POST https://httpbin.org/post Content-Type:application/json",
			wantBody: `{"name":"zombie"}`,
		},
		{
			name:     "with auth",
			curl:     "curl -u admin:secret https://api.example.com/admin",
			wantArgs: "https://api.example.com/admin --auth admin:secret",
			wantBody: "",
		},
		{
			name:     "with follow and insecure",
			curl:     "curl -L -k https://example.com",
			wantArgs: "https://example.com --follow --verify no",
			wantBody: "",
		},
		{
			name:     "compressed and silent",
			curl:     "curl --compressed -s https://api.example.com/data",
			wantArgs: "https://api.example.com/data --compress",
			wantBody: "",
		},
		{
			name:     "multiline with backslash",
			curl:     "curl -X GET https://example.com \\\n  -H \"Accept: application/json\" \\\n  -L",
			wantArgs: "GET https://example.com Accept:application/json --follow",
			wantBody: "",
		},
		{
			name:     "PUT with bearer token header",
			curl:     `curl -X PUT https://api.example.com/users/1 -H "Authorization: Bearer token123" -H "Content-Type: application/json" -d '{"name":"updated"}'`,
			wantArgs: "PUT https://api.example.com/users/1 Authorization:Bearer token123 Content-Type:application/json",
			wantBody: `{"name":"updated"}`,
		},
		{
			name:     "just URL without curl prefix",
			curl:     "https://httpbin.org/get",
			wantArgs: "https://httpbin.org/get",
			wantBody: "",
		},
		{
			name:     "user exact curl with backslash continuation",
			curl:     "curl -X POST https://api.example.com/login \\\n  -H \"Content-Type: application/json\" \\\n  -H \"Authorization: Bearer token\" \\\n  -d '{\"user\":\"admin\"}'",
			wantArgs: "POST https://api.example.com/login Content-Type:application/json Authorization:Bearer token",
			wantBody: `{"user":"admin"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, body := CurlToXhArgs(tt.curl)
			gotArgs := strings.Join(args, " ")
			if gotArgs != tt.wantArgs {
				t.Errorf("args:\n  got:  %q\n  want: %q", gotArgs, tt.wantArgs)
			}
			if body != tt.wantBody {
				t.Errorf("body:\n  got:  %q\n  want: %q", body, tt.wantBody)
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple",
			input: "GET https://example.com",
			want:  []string{"GET", "https://example.com"},
		},
		{
			name:  "single quotes",
			input: `-d '{"key": "value"}'`,
			want:  []string{"-d", `{"key": "value"}`},
		},
		{
			name:  "double quotes",
			input: `-H "Content-Type: application/json"`,
			want:  []string{"-H", "Content-Type: application/json"},
		},
		{
			name:  "backslash continuation",
			input: "-X POST \\\nhttps://example.com",
			want:  []string{"-X", "POST", "https://example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tokenize(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("len: got %d, want %d\n  got:  %v\n  want: %v", len(got), len(tt.want), got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("token[%d]: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
