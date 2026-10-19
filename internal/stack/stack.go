// Package stack loads inputs without resolving or retaining secret values.
package stack

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Haze-dev1/Olympus/internal/schema"
)

type Server struct {
	Host, User     string
	SSHKey         string `json:"ssh_key"`
	Port, Keep     int
	ConfirmTimeout string `json:"confirm_timeout"`
}
type MCP struct {
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	URL     string            `json:"url,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}
type Channel struct {
	Token string
	Allow []string
}
type Tool struct {
	Recipe   string
	ImageTag string `json:"image_tag"`
	MCPs     []string
	Skills   any
	Secrets  map[string]string
	Channels map[string]Channel
	Ports    struct{ Public []int }
}
type Stack struct {
	Inputs       struct{ Recipes string }
	Server       Server
	MCPs         map[string]MCP
	Skills       []string
	Agents, Apps map[string]Tool
}

func (s Stack) Tools() map[string]Tool {
	out := map[string]Tool{}
	for k, v := range s.Agents {
		out[k] = v
	}
	for k, v := range s.Apps {
		out[k] = v
	}
	return out
}

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,30}$`)
var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func ValidName(s string) bool { return namePattern.MatchString(s) }

func Load(file string) (*Stack, *schema.Document) {
	b, err := os.ReadFile(file)
	if err != nil {
		d := &schema.Document{File: file}
		d.Add("", "cannot read stack file")
		return nil, d
	}
	return Parse(file, b)
}
func Parse(file string, b []byte) (*Stack, *schema.Document) {
	d := schema.Parse(file, b, "stack")
	if len(d.Errors) > 0 {
		return nil, d
	}
	var s Stack
	if d.Decode(&s) != nil {
		d.Add("", "cannot decode stack")
		return nil, d
	}
	if len(s.Agents)+len(s.Apps) == 0 {
		d.Add("", "at least one tool is required")
	}
	for _, group := range []string{"agents", "apps"} {
		tools := s.Agents
		if group == "apps" {
			tools = s.Apps
		}
		for _, name := range schema.Keys(tools) {
			p := schema.Child("/"+group, name)
			t := tools[name]
			if !ValidName(name) {
				d.Add(p, "tool name must match ^[a-z][a-z0-9-]{0,30}$")
			}
			if _, exists := s.Agents[name]; group == "apps" && exists {
				d.Add(p, "tool name is already used under agents")
			}
			for i, m := range t.MCPs {
				if _, ok := s.MCPs[m]; !ok {
					d.Add(fmt.Sprintf("%s/mcps/%d", p, i), "MCP reference is not defined under mcps")
				}
			}
			for _, key := range schema.Keys(t.Secrets) {
				if !fullReference(t.Secrets[key]) {
					d.Add(schema.Child(p+"/secrets", key), "secret must be a single ${NAME} reference")
				}
			}
			for _, key := range schema.Keys(t.Channels) {
				if !fullReference(t.Channels[key].Token) {
					d.Add(schema.Child(p+"/channels", key)+"/token", "channel token must be a single ${NAME} reference")
				}
			}
			if skills, ok := t.Skills.([]any); ok {
				for i, v := range skills {
					found := false
					for _, decl := range s.Skills {
						if decl == v {
							found = true
						}
					}
					if !found {
						d.Add(fmt.Sprintf("%s/skills/%d", p, i), "skill is not declared at top level")
					}
				}
			}
		}
	}
	for _, name := range schema.Keys(s.MCPs) {
		p := schema.Child("/mcps", name)
		m := s.MCPs[name]
		if (m.Command == "") == (m.URL == "") {
			d.Add(p, "MCP must specify exactly one of command or url")
		}
		if m.URL != "" {
			u, err := url.Parse(m.URL)
			if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
				d.Add(p+"/url", "expected HTTP(S) URL without credentials")
			}
			if len(m.Args) > 0 || len(m.Env) > 0 {
				d.Add(p, "HTTP MCP cannot have args or env")
			}
		}
		if m.Command != "" && len(m.Headers) > 0 {
			d.Add(p, "stdio MCP cannot have headers")
		}
	}
	if s.Server.ConfirmTimeout != "" {
		dur, err := time.ParseDuration(s.Server.ConfirmTimeout)
		if err != nil || dur <= 0 {
			d.Add("/server/confirm_timeout", "expected positive duration")
		}
	}
	return &s, d
}
func fullReference(s string) bool { a := schema.Variables(s); return len(a) == 1 && s == "${"+a[0]+"}" }

// CheckEnv scans names only. Values are neither stored nor returned to evaluation.
// Process environment fallback is deliberately absent in the offline slice.
func CheckEnv(d *schema.Document, stackFile string) {
	if len(d.References) == 0 {
		return
	}
	file := filepath.Join(filepath.Dir(stackFile), ".env")
	f, err := os.Open(file)
	names := map[string]bool{}
	if err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 4096), 4<<20)
		line := 0
		for scanner.Scan() {
			line++
			s := strings.TrimSpace(scanner.Text())
			if s == "" || strings.HasPrefix(s, "#") {
				continue
			}
			key, _, ok := strings.Cut(s, "=")
			key = strings.TrimSpace(key)
			if !ok || !envName.MatchString(key) {
				d.Errors = append(d.Errors, schema.Diagnostic{File: file, Line: line, Column: 1, Message: "expected NAME=value"})
				continue
			}
			names[key] = true
		}
		if scanner.Err() != nil {
			d.Errors = append(d.Errors, schema.Diagnostic{File: file, Line: line + 1, Column: 1, Message: "cannot read environment names"})
		}
	}
	for _, name := range schema.Keys(d.References) {
		if !names[name] {
			for _, p := range d.References[name] {
				d.Add(p, "missing variable "+name+" in "+file)
			}
		}
	}
}
