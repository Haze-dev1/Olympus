// Package eval builds generations from validated inputs. It reads no environment,
// network, clock, randomness, or secret values.
package eval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Haze-dev1/Olympus/internal/encode"
	"github.com/Haze-dev1/Olympus/internal/generation"
	"github.com/Haze-dev1/Olympus/internal/recipe"
	"github.com/Haze-dev1/Olympus/internal/schema"
	"github.com/Haze-dev1/Olympus/internal/stack"
)

const Version = "0.1.0-dev"

type Service struct {
	Image       string            `json:"image"`
	User        string            `json:"user,omitempty"`
	Restart     string            `json:"restart"`
	Ports       []string          `json:"ports,omitempty"`
	Volumes     []Mount           `json:"volumes,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	EnvFile     []string          `json:"env_file,omitempty"`
	Labels      map[string]string `json:"labels"`
}
type Mount struct {
	Type     string `json:"type"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"read_only,omitempty"`
}
type Compose struct {
	Name     string                       `json:"name"`
	Services map[string]Service           `json:"services"`
	Volumes  map[string]map[string]string `json:"volumes,omitempty"`
}

// Inputs are collected before evaluation. Keeping file IO outside Build makes
// the evaluator pure and lets tests supply exactly the same bytes twice.
type Inputs struct {
	Revision  string
	Images    map[string]string
	Stack     *stack.Stack
	Recipes   map[string]*recipe.Recipe
	Templates map[string]map[string][]byte
	Skills    map[string]map[string][]byte
}

func Load(s *stack.Stack, root, stackDir string) (Inputs, []schema.Diagnostic) {
	in := Inputs{Stack: s, Recipes: map[string]*recipe.Recipe{}, Templates: map[string]map[string][]byte{}, Skills: map[string]map[string][]byte{}}
	var errors []schema.Diagnostic
	for _, name := range schema.Keys(s.Tools()) {
		t := s.Tools()[name]
		if in.Recipes[t.Recipe] != nil {
			continue
		}
		r, d := recipe.Load(root, t.Recipe)
		errors = append(errors, d.Errors...)
		if len(d.Errors) > 0 {
			continue
		}
		in.Recipes[t.Recipe] = r
		in.Templates[t.Recipe] = map[string][]byte{}
		for _, tpl := range r.Templates {
			b, e := recipe.ReadFile(r.Dir, tpl.Src)
			if e != nil {
				d.Add("/templates", "cannot read confined template source")
			} else {
				in.Templates[t.Recipe][tpl.Src] = b
			}
		}
		errors = append(errors, d.Errors...)
	}
	for _, skill := range s.Skills {
		if !recipe.SafeRelative(skill) {
			errors = append(errors, schema.Diagnostic{File: "stack.yaml", Line: 1, Column: 1, Message: "skill must be a confined relative path"})
			continue
		}
		files := map[string][]byte{}
		rootDir := filepath.Join(stackDir, skill)
		e := filepath.WalkDir(rootDir, func(p string, d fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			rel, e := filepath.Rel(stackDir, p)
			if e != nil {
				return e
			}
			if d.Type()&fs.ModeSymlink != 0 {
				return fmt.Errorf("symlink in skill")
			}
			if d.IsDir() {
				return nil
			}
			b, e := recipe.ReadFile(stackDir, rel)
			if e != nil {
				return e
			}
			dest, e := filepath.Rel(rootDir, p)
			if e != nil {
				return e
			}
			files[filepath.ToSlash(dest)] = b
			return nil
		})
		if e != nil || files["SKILL.md"] == nil {
			errors = append(errors, schema.Diagnostic{File: "stack.yaml", Line: 1, Column: 1, Message: "cannot load confined skill with SKILL.md"})
		} else {
			in.Skills[skill] = files
		}
	}
	return in, errors
}
func Build(in Inputs) (*generation.Generation, error) {
	g := &generation.Generation{Files: map[string][]byte{}, Manifest: generation.Manifest{Version: 1, Olym: Version, Inputs: map[string]string{}, Tools: map[string]generation.Tool{}}}
	compose := Compose{Name: "olympus", Services: map[string]Service{}, Volumes: map[string]map[string]string{}}
	catalog := map[string]any{}
	for _, key := range schema.Keys(in.Recipes) {
		catalog[key] = struct {
			Recipe    *recipe.Recipe
			Templates map[string][]byte
		}{in.Recipes[key], in.Templates[key]}
	}
	b, err := encode.Canonical(catalog)
	if err != nil {
		return nil, err
	}
	g.Manifest.Inputs["recipes"] = generation.Hash(b)
	if in.Revision != "" {
		g.Manifest.Inputs["recipes"] = in.Revision
	}
	for _, name := range schema.Keys(in.Stack.Tools()) {
		entry := in.Stack.Tools()[name]
		r := in.Recipes[entry.Recipe]
		if r == nil {
			return nil, fmt.Errorf("%s: recipe was not loaded", name)
		}
		service := Service{Image: r.Image, User: r.User, Restart: "unless-stopped", Environment: r.Env, Labels: map[string]string{"org.olympus.tool": name}}
		if in.Images != nil {
			pinned, ok := in.Images[name]
			if !ok {
				return nil, fmt.Errorf("lock lacks tool %s; run olym update", name)
			}
			service.Image = pinned
		} else {
			image, err := Image(r.Image, entry.ImageTag)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			service.Image = image
		}

		for _, port := range r.Ports {
			service.Ports = append(service.Ports, fmt.Sprintf("127.0.0.1:%d:%d", port, port))
		}
		for _, p := range entry.Ports.Public {
			found := false
			for _, port := range r.Ports {
				if p == port {
					found = true
				}
			}
			if !found {
				return nil, fmt.Errorf("%s: public port is not declared by recipe", name)
			}
		}
		// This slice always binds localhost, even if public ports were requested.
		if len(entry.Ports.Public) > 0 {
			return nil, fmt.Errorf("%s: public port builds are outside the offline slice", name)
		}
		for _, volume := range schema.Keys(r.Volumes) {
			source := name + "_" + volume
			compose.Volumes[source] = map[string]string{"name": "olympus_" + name + "_" + volume}
			service.Volumes = append(service.Volumes, Mount{Type: "volume", Source: source, Target: r.Volumes[volume]})
		}
		secretNames := []string{}
		runtimeVars := map[string]string{}
		for _, secret := range schema.Keys(entry.Secrets) {
			decl, ok := r.Secrets[secret]
			if !ok {
				return nil, fmt.Errorf("%s: secret is not declared by recipe", name)
			}
			vars := schema.Variables(entry.Secrets[secret])
			if len(vars) != 1 {
				return nil, fmt.Errorf("%s: invalid secret reference", name)
			}
			if existing, ok := runtimeVars[vars[0]]; ok && existing != decl.Env {
				return nil, fmt.Errorf("%s: ambiguous secret environment mapping", name)
			}
			runtimeVars[vars[0]] = decl.Env
			secretNames = append(secretNames, secret)
		}
		if len(secretNames) > 0 {
			service.EnvFile = []string{"/var/lib/olympus/secrets/" + name + "/environment"}
		}
		mcps := map[string]any{}
		mcpNames := entry.MCPs
		if r.Supports.MCPs == nil {
			mcpNames = nil
		}
		for _, mcpName := range mcpNames {
			m, ok := in.Stack.MCPs[mcpName]
			if !ok {
				return nil, fmt.Errorf("%s: undefined MCP reference", name)
			}
			m.Env, err = runtimeReferences(m.Env, runtimeVars)
			if err != nil {
				return nil, fmt.Errorf("%s: MCP secret lacks a declared environment mapping", name)
			}
			m.Headers, err = runtimeReferences(m.Headers, runtimeVars)
			if err != nil {
				return nil, fmt.Errorf("%s: MCP secret lacks a declared environment mapping", name)
			}
			b, err := encode.Canonical(m)
			if err != nil {
				return nil, err
			}
			var obj any
			if err = json.Unmarshal(b, &obj); err != nil {
				return nil, err
			}
			mcps[mcpName] = obj
		}
		files := map[string][]byte{}
		// Templates receive only non-secret settings and translated MCP references.
		data := struct {
			Name   string
			Recipe string
			MCPs   map[string]any
			Skills any
		}{name, entry.Recipe, mcps, entry.Skills}
		for _, tpl := range r.Templates {
			if !r.Confined(tpl.Dst) || !recipe.SafeRelative(tpl.Src) {
				return nil, fmt.Errorf("%s: unsafe template path", name)
			}
			source, ok := in.Templates[entry.Recipe][tpl.Src]
			if !ok {
				return nil, fmt.Errorf("%s: missing template source", name)
			}
			t, e := template.New(tpl.Src).Option("missingkey=error").Parse(string(source))
			if e != nil {
				return nil, fmt.Errorf("%s: invalid template", name)
			}
			var buf bytes.Buffer
			if e = t.Execute(&buf, data); e != nil {
				return nil, fmt.Errorf("%s: template execution failed", name)
			}
			files[tpl.Dst] = buf.Bytes()
		}
		managed := map[string]map[string]any{}
		formats := map[string]string{}
		set := func(m recipe.Managed, value any) error {
			if !r.Confined(m.File) {
				return fmt.Errorf("%s: unsafe managed file path", name)
			}
			if previous := formats[m.File]; previous != "" && previous != m.Format {
				return fmt.Errorf("%s: conflicting config formats", name)
			}
			formats[m.File] = m.Format
			doc, ok := managed[m.File]
			if !ok {
				doc = map[string]any{}
				if seed, ok := files[m.File]; ok {
					var e error
					doc, e = encode.Unmarshal(m.Format, seed)
					if e != nil {
						return fmt.Errorf("%s: invalid structured template", name)
					}
				}
				managed[m.File] = doc
			}
			return setKey(doc, m.Key, value)
		}
		if r.Supports.MCPs != nil {
			if err = set(*r.Supports.MCPs, mcps); err != nil {
				return nil, err
			}
		} else if len(entry.MCPs) > 0 {
			g.Warnings = append(g.Warnings, name+": recipe does not support MCP servers; skipped")
		}
		for _, channel := range schema.Keys(entry.Channels) {
			c := entry.Channels[channel]
			if len(c.Allow) == 0 {
				return nil, fmt.Errorf("%s: channel requires a non-empty allow-list", name)
			}
			support, ok := r.Supports.Channels[channel]
			if !ok {
				return nil, fmt.Errorf("%s: channel not supported by recipe", name)
			}
			// Per-tool channel secret delivery remains outside this slice.
			_ = support
			return nil, fmt.Errorf("%s: channel rendering is outside the offline slice", name)
		}
		selected := in.Stack.Skills
		if a, ok := entry.Skills.([]any); ok {
			selected = nil
			for _, v := range a {
				selected = append(selected, v.(string))
			}
		} else if entry.Skills == nil {
			selected = nil
		}
		if len(selected) > 0 && r.Supports.Skills == nil {
			g.Warnings = append(g.Warnings, name+": recipe does not support skills; skipped")
		} else if r.Supports.Skills != nil {
			for _, skill := range selected {
				skillFiles, ok := in.Skills[skill]
				if !ok {
					return nil, fmt.Errorf("%s: skill was not loaded", name)
				}
				for _, rel := range schema.Keys(skillFiles) {
					target := path.Join(r.Supports.Skills.Dir, path.Base(skill), rel)
					if !r.Confined(target) || files[target] != nil {
						return nil, fmt.Errorf("%s: unsafe or colliding skill destination", name)
					}
					files[target] = skillFiles[rel]
				}
			}
		}
		for _, p := range schema.Keys(managed) {
			files[p], err = encode.Marshal(formats[p], managed[p])
			if err != nil {
				return nil, fmt.Errorf("%s: cannot encode structured config", name)
			}
		}
		tool := generation.Tool{Image: service.Image, Files: map[string]string{}, Secrets: secretNames}
		if r.Health.HTTP != nil || len(r.Health.Command) > 0 {
			tool.Health = &generation.Health{Command: r.Health.Command, Timeout: r.Health.Timeout}
			if r.Health.HTTP != nil {
				tool.Health.HTTP = &generation.HTTPCheck{Port: r.Health.HTTP.Port, Path: r.Health.HTTP.Path}
			}
		}
		for _, p := range schema.Keys(files) {
			if !r.Confined(p) {
				return nil, fmt.Errorf("%s: unsafe generated path", name)
			}
			rel := path.Join("tools", name, "files", strings.TrimPrefix(p, "/"))
			g.Files[rel] = files[p]
			tool.Files[p] = generation.Hash(files[p])
			service.Volumes = append(service.Volumes, Mount{Type: "bind", Source: "./" + rel, Target: p, ReadOnly: true})
		}
		tool.Hash, err = generation.ToolHash(files, service)
		if err != nil {
			return nil, err
		}
		g.Manifest.Tools[name] = tool
		compose.Services[name] = service
	}
	g.Files["compose.yaml"], err = encode.Marshal("yaml", compose)
	if err != nil {
		return nil, err
	}
	if err = g.Finish(); err != nil {
		return nil, err
	}
	return g, nil
}
func runtimeReferences(values map[string]string, names map[string]string) (map[string]string, error) {
	out := map[string]string{}
	for _, key := range schema.Keys(values) {
		var replacements []string
		for _, name := range schema.Variables(values[key]) {
			env, ok := names[name]
			if !ok {
				return nil, fmt.Errorf("undeclared runtime secret")
			}
			replacements = append(replacements, "${"+name+"}", "${"+env+"}")
		}
		out[key] = strings.NewReplacer(replacements...).Replace(values[key])
	}
	return out, nil
}

func setKey(doc map[string]any, key string, value any) error {
	parts := strings.Split(key, ".")
	for _, part := range parts[:len(parts)-1] {
		if part == "" {
			return fmt.Errorf("invalid managed key")
		}
		next, ok := doc[part]
		if !ok {
			child := map[string]any{}
			doc[part] = child
			doc = child
			continue
		}
		child, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("managed key crosses a scalar")
		}
		doc = child
	}
	if parts[len(parts)-1] == "" {
		return fmt.Errorf("empty managed key")
	}
	doc[parts[len(parts)-1]] = value
	return nil
}

// Image applies the stack's optional tag override before lock resolution.
func Image(base, tag string) (string, error) {
	if tag == "" {
		return base, nil
	}
	if strings.Contains(base, "@") {
		return "", fmt.Errorf("cannot override tag of digest image")
	}
	if colon := strings.LastIndex(base, ":"); colon > strings.LastIndex(base, "/") {
		base = base[:colon]
	}
	return base + ":" + tag, nil
}
