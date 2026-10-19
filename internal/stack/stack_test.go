package stack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const valid = "server:\n  host: example.test\nagents:\n  assistant:\n    recipe: observatory\n"

func TestValidation(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"valid", valid, ""},
		{"unknown root", valid + "surprise: true\n", "unknown field"},
		{"unknown nested", strings.Replace(valid, "host: example.test", "host: example.test\n  wrong: 1", 1), "unknown field"},
		{"name", strings.Replace(valid, "assistant:", "Bad_Name:", 1), "tool name"},
		{"duplicate name", valid + "apps:\n  assistant: {recipe: observatory}\n", "already used"},
		{"missing recipe", strings.Replace(valid, "recipe: observatory", "mcps: []", 1), "missing required field recipe"},
		{"empty tools", "server: {host: example.test}\n", "at least one tool"},
		{"type trap integer", "server: {host: 08}\nagents: {a: {recipe: observatory}}", "expected string"},
		{"type trap bool", "server: {host: false}\nagents: {a: {recipe: observatory}}", "expected string"},
		{"type trap on", "server: {host: example.test, port: on}\nagents: {a: {recipe: observatory}}", "expected integer"},
		{"type trap no", "server: {host: example.test, port: no}\nagents: {a: {recipe: observatory}}", "expected integer"},
		{"out of range", strings.Replace(valid, "host: example.test", "host: example.test\n  port: 65536", 1), "above maximum"},
		{"unknown MCP", strings.Replace(valid, "recipe: observatory", "recipe: observatory\n    mcps: [missing]", 1), "not defined"},
		{"bad interpolation", strings.Replace(valid, "example.test", "${HOST}", 1), "secret-capable"},
		{"interpolated key", valid + "${BAD}: value\n", "forbidden in keys"},
		{"malformed variable", valid + "mcps:\n  github: {command: mcp, env: {AUTH: '${bad-name}'}}", "invalid variable"},
		{"inline secret", valid + "    secrets: {login: inline-forbidden}\n", "single ${NAME}"},
		{"empty allow", valid + "    channels: {telegram: {token: '${TG}', allow: []}}\n", "list must not be empty"},
		{"both transports", valid + "mcps:\n  docs: {command: run, url: https://example.test}\n", "exactly one"},
		{"no transport", valid + "mcps:\n  docs: {args: [stdio]}\n", "exactly one"},
		{"url", valid + "mcps:\n  docs: {url: 'file:///etc/passwd'}\n", "HTTP(S)"},
		{"alias", valid + "mcps: &shared {}\napps: *shared\n", "unsupported YAML node"},
		{"tag", valid + "mcps: !!python/object {}\n", "unsupported YAML node"},
		{"multiple documents", valid + "---\nserver: {}\n", "one YAML document"},
		{"duplicate key", valid + "server: {host: duplicate}\n", "invalid YAML"},
		{"syntax", "server: [\n", "invalid YAML"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, d := Parse("stack.yaml", []byte(tc.body))
			messages := []string{}
			for _, e := range d.Errors {
				if e.Line < 1 || e.Column < 1 || e.File != "stack.yaml" {
					t.Fatalf("missing position: %#v", e)
				}
				messages = append(messages, e.Message)
			}
			joined := strings.Join(messages, "\n")
			if tc.want == "" {
				if joined != "" {
					t.Fatal(joined)
				}
			} else if !strings.Contains(joined, tc.want) {
				t.Fatalf("want %q, got %q", tc.want, joined)
			}
		})
	}
}
func TestAllErrorsHaveExactPositions(t *testing.T) {
	_, d := Parse("stack.yaml", []byte("server:\n  host: 123\n  port: bad\nagents:\n  a: {recipe: observatory}\n"))
	if len(d.Errors) != 2 {
		t.Fatalf("got %v", d.Errors)
	}
	if d.Errors[0].Line != 2 || d.Errors[0].Column != 9 || d.Errors[1].Line != 3 || d.Errors[1].Column != 9 {
		t.Fatalf("wrong locations: %v", d.Errors)
	}
}
func TestEnvNamesOnly(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "stack.yaml")
	input := valid + "    secrets: {one: '${AUTH}', two: '${AUTH}'}\n"
	_, d := Parse(file, []byte(input))
	CheckEnv(d, file)
	if len(d.Errors) != 2 {
		t.Fatalf("missing every use: %v", d.Errors)
	}
	for _, e := range d.Errors {
		if !strings.Contains(e.Message, filepath.Join(dir, ".env")) || !strings.Contains(e.Message, "AUTH") {
			t.Fatal(e)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("# names only\nAUTH=OLYM_SENTINEL_private_${NOT_EXPANDED}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, d = Parse(file, []byte(input))
	CheckEnv(d, file)
	if len(d.Errors) != 0 {
		t.Fatal(d.Errors)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("bad name=OLYM_SENTINEL_private\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, d = Parse(file, []byte(input))
	CheckEnv(d, file)
	if len(d.Errors) != 3 {
		t.Fatal(d.Errors)
	}
	for _, e := range d.Errors {
		if strings.Contains(e.Error(), "OLYM_SENTINEL") {
			t.Fatal("secret leaked")
		}
	}
}
