package eval

import (
	"reflect"
	"testing"
)

func TestTranslateMCPRemoteTransport(t *testing.T) {
	cases := []struct {
		name  string
		style string
		in    map[string]any
		want  map[string]any
	}{
		{
			name:  "claude-code stdio stays unchanged",
			style: "claude-code",
			in:    map[string]any{"command": "npx", "args": []any{"-y", "server"}},
			want:  map[string]any{"command": "npx", "args": []any{"-y", "server"}},
		},
		{
			name:  "claude-code defaults to http",
			style: "claude-code",
			in:    map[string]any{"url": "https://mcp.example.test/mcp"},
			want:  map[string]any{"type": "http", "url": "https://mcp.example.test/mcp"},
		},
		{
			name:  "claude-code sse",
			style: "claude-code",
			in:    map[string]any{"url": "https://mcp.example.test/sse", "transport": "sse"},
			want:  map[string]any{"type": "sse", "url": "https://mcp.example.test/sse"},
		},
		{
			name:  "gemini-cli stdio stays unchanged",
			style: "gemini-cli",
			in:    map[string]any{"command": "npx", "args": []any{"-y", "server"}},
			want:  map[string]any{"command": "npx", "args": []any{"-y", "server"}},
		},
		{
			name:  "gemini-cli defaults to httpUrl",
			style: "gemini-cli",
			in:    map[string]any{"url": "https://mcp.example.test/mcp"},
			want:  map[string]any{"httpUrl": "https://mcp.example.test/mcp"},
		},
		{
			name:  "gemini-cli sse keeps url",
			style: "gemini-cli",
			in:    map[string]any{"url": "https://mcp.example.test/sse", "transport": "sse"},
			want:  map[string]any{"url": "https://mcp.example.test/sse"},
		},
		{
			name:  "hermes keeps the generic shape",
			style: "hermes",
			in:    map[string]any{"url": "https://mcp.example.test/mcp", "transport": "sse"},
			want:  map[string]any{"url": "https://mcp.example.test/mcp"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := translateMCP(tc.in, tc.style)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}
