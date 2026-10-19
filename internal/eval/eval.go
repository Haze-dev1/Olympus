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
	Image       string             `json:"image"`
	User        string             `json:"user,omitempty"`
	Restart     string             `json:"restart"`
	Command     []string           `json:"command,omitempty"`
	Entrypoint  []string           `json:"entrypoint,omitempty"`
	Ports       []string           `json:"ports,omitempty"`
	Volumes     []Mount            `json:"volumes,omitempty"`
	Environment map[string]string  `json:"environment,omitempty"`
	EnvFile     []string           `json:"env_file,omitempty"`
	Build       *ServiceBuild      `json:"build,omitempty"`
	DependsOn   map[string]Depends `json:"depends_on,omitempty"`
	Labels      map[string]string  `json:"labels"`
}
type ServiceBuild struct {
	Context    string            `json:"context"`
	Dockerfile string            `json:"dockerfile"`
	Args       map[string]string `json:"args,omitempty"`
}
type Depends struct {
	Condition string `json:"condition"`
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
		if r.Build != nil {
			b, e := recipe.ReadFile(r.Dir, r.Build.Src)
			if e != nil {
				d.Add("/build/src", "cannot read confined build source")
			} else {
				in.Templates[t.Recipe][r.Build.Src] = b
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
		environment := map[string]string{}
		for k, v := range r.Env {
			environment[k] = v
		}
		service := Service{Image: r.Image, User: r.User, Restart: "unless-stopped", Command: r.Command, Environment: environment, Labels: map[string]string{"org.olympus.tool": name}}
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
		baseRef := service.Image

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
			var obj map[string]any
			if err = json.Unmarshal(b, &obj); err != nil {
				return nil, err
			}
			translated, err := translateMCP(obj, r.Supports.MCPs.Style)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			mcps[mcpName] = translated
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
		var buildDst, buildBase string
		if r.Build != nil {
			source, ok := in.Templates[entry.Recipe][r.Build.Src]
			if !ok {
				return nil, fmt.Errorf("%s: missing build source", name)
			}
			if !r.Confined(r.Build.Dst) || files[r.Build.Dst] != nil {
				return nil, fmt.Errorf("%s: unsafe or colliding build destination", name)
			}
			files[r.Build.Dst] = source
			service.Image = builtImage(name, source, baseRef)
			buildDst, buildBase = r.Build.Dst, baseRef
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
					if doc == nil {
						doc = map[string]any{}
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
			tokenEnv := support.Env["token"]
			secretName := ""
			for _, sn := range schema.Keys(r.Secrets) {
				if r.Secrets[sn].Env == tokenEnv {
					secretName = sn
				}
			}
			if secretName == "" {
				return nil, fmt.Errorf("%s: channel token environment variable is not a declared secret", name)
			}
			provided, ok := entry.Secrets[secretName]
			if !ok {
				return nil, fmt.Errorf("%s: channel token secret %q is not provided by the stack", name, secretName)
			}
			if schema.Variables(provided)[0] != schema.Variables(c.Token)[0] {
				return nil, fmt.Errorf("%s: channel token does not match secret %q", name, secretName)
			}
			if allowEnv := support.Env["allow"]; allowEnv != "" {
				service.Environment[allowEnv] = strings.Join(c.Allow, ",")
			}
		}
		var skillCopies []string
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
					if r.Supports.Skills.Mode == "copy" {
						if !recipe.ShellSafe(target) {
							return nil, fmt.Errorf("%s: skill destination is not shell-safe", name)
						}
						skillCopies = append(skillCopies, target)
					}
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
			tool.Files[p] = generation.Hash(files[p])
		}
		tool.Hash, err = generation.ToolHash(files, service)
		if err != nil {
			return nil, err
		}
		for _, p := range schema.Keys(files) {
			rel := path.Join("store", tool.Hash, "files", strings.TrimPrefix(p, "/"))
			g.Files[rel] = files[p]
			service.Volumes = append(service.Volumes, Mount{Type: "bind", Source: "/var/lib/olympus/" + rel, Target: p, ReadOnly: true})
		}
		// Compose build derives its context from the bind mount just created for
		// the Dockerfile, so it follows the store layout without depending on it.
		if buildDst != "" {
			context, dockerfile := "", ""
			for _, m := range service.Volumes {
				if m.Target == buildDst {
					context, dockerfile = path.Dir(m.Source), path.Base(m.Source)
				}
			}
			if context == "" {
				return nil, fmt.Errorf("%s: build file was not mounted", name)
			}
			service.Build = &ServiceBuild{Context: context, Dockerfile: dockerfile, Args: map[string]string{"BASE_IMAGE": buildBase}}
		}
		// Writable managed files and copied skills are delivered by a one-shot
		// init service: Compose cannot copy, so the init container mounts the
		// rendered file read-only and copies it into the tool volume first.
		copies := append([]string{}, r.Writable...)
		copies = append(copies, skillCopies...)
		if len(copies) > 0 {
			initName := name + "-init"
			if _, exists := compose.Services[initName]; exists {
				return nil, fmt.Errorf("%s: init service name collides", name)
			}
			init := Service{
				Image:   service.Image,
				User:    r.User,
				Restart: "no",
				Labels:  map[string]string{"org.olympus.tool": name, "org.olympus.role": "init"},
			}
			for _, m := range service.Volumes {
				if m.Type == "volume" {
					init.Volumes = append(init.Volumes, m)
				}
			}
			var script strings.Builder
			for i, dst := range copies {
				source := ""
				for j := 0; j < len(service.Volumes); j++ {
					if m := service.Volumes[j]; m.Type == "bind" && m.Target == dst {
						source = m.Source
						service.Volumes = append(service.Volumes[:j], service.Volumes[j+1:]...)
						break
					}
				}
				if source == "" {
					return nil, fmt.Errorf("%s: writable path is not a generated file", name)
				}
				staging := fmt.Sprintf("/olympus-init/%d", i)
				init.Volumes = append(init.Volumes, Mount{Type: "bind", Source: source, Target: staging, ReadOnly: true})
				fmt.Fprintf(&script, "mkdir -p '%s' && cp -a '%s' '%s'\n", path.Dir(dst), staging, dst)
			}
			init.Entrypoint = []string{"/bin/sh", "-c", script.String()}
			compose.Services[initName] = init
			service.DependsOn = map[string]Depends{initName: {Condition: "service_completed_successfully"}}
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

// translateMCP rewrites the generic stack MCP entry into the tool's own config
// shape. Secret values are never written: they become the tool's runtime
// reference syntax (Hermes ${VAR}, OpenCode {env:VAR}) or an environment
// variable name (Codex).
func translateMCP(obj map[string]any, style string) (map[string]any, error) {
	command, hasCommand := obj["command"]
	url, hasURL := obj["url"]
	switch style {
	case "hermes":
		out := map[string]any{}
		for _, key := range []string{"command", "args", "env", "url", "headers"} {
			if v, ok := obj[key]; ok {
				out[key] = v
			}
		}
		return out, nil
	case "codex":
		if hasURL {
			out := map[string]any{"url": url}
			headers, _ := obj["headers"].(map[string]any)
			static := map[string]any{}
			envHeaders := map[string]any{}
			for _, k := range schema.Keys(headers) {
				if name, ok := soleReference(headers[k]); ok {
					envHeaders[k] = name
				} else if s, ok := headers[k].(string); ok {
					static[k] = s
				} else {
					return nil, fmt.Errorf("MCP header %s is not a string", k)
				}
			}
			if len(static) > 0 {
				out["http_headers"] = static
			}
			if name, ok := envHeaders["Authorization"]; ok && len(envHeaders) == 1 && len(static) == 0 {
				out["bearer_token_env_var"] = name
			} else if len(envHeaders) > 0 {
				out["env_http_headers"] = envHeaders
			}
			return out, nil
		}
		if !hasCommand {
			return nil, fmt.Errorf("MCP has neither command nor url")
		}
		out := map[string]any{"command": command}
		if args, ok := obj["args"]; ok {
			out["args"] = args
		}
		literal := map[string]any{}
		names := []any{}
		env, _ := obj["env"].(map[string]any)
		for _, k := range schema.Keys(env) {
			if name, ok := soleReference(env[k]); ok {
				names = append(names, name)
			} else if s, ok := env[k].(string); ok {
				literal[k] = s
			} else {
				return nil, fmt.Errorf("MCP environment value %s is not a string", k)
			}
		}
		if len(names) > 0 {
			out["env_vars"] = names
		}
		if len(literal) > 0 {
			out["env"] = literal
		}
		return out, nil
	case "opencode":
		if hasURL {
			out := map[string]any{"type": "remote", "url": url}
			headers, _ := obj["headers"].(map[string]any)
			if len(headers) > 0 {
				h := map[string]any{}
				for _, k := range schema.Keys(headers) {
					if name, ok := soleReference(headers[k]); ok {
						h[k] = "{env:" + name + "}"
					} else if s, ok := headers[k].(string); ok {
						h[k] = s
					} else {
						return nil, fmt.Errorf("MCP header %s is not a string", k)
					}
				}
				out["headers"] = h
			}
			return out, nil
		}
		if !hasCommand {
			return nil, fmt.Errorf("MCP has neither command nor url")
		}
		list := []any{command}
		if args, ok := obj["args"].([]any); ok {
			list = append(list, args...)
		}
		out := map[string]any{"type": "local", "command": list}
		env, _ := obj["env"].(map[string]any)
		if len(env) > 0 {
			e := map[string]any{}
			for _, k := range schema.Keys(env) {
				if name, ok := soleReference(env[k]); ok {
					e[k] = "{env:" + name + "}"
				} else if s, ok := env[k].(string); ok {
					e[k] = s
				} else {
					return nil, fmt.Errorf("MCP environment value %s is not a string", k)
				}
			}
			out["environment"] = e
		}
		return out, nil
	}
	return nil, fmt.Errorf("unknown MCP style %q", style)
}

// soleReference accepts exactly ${VAR} or "Bearer ${VAR}" and returns VAR.
func soleReference(v any) (string, bool) {
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	vars := schema.Variables(s)
	if len(vars) != 1 {
		return "", false
	}
	if s != "${"+vars[0]+"}" && s != "Bearer ${"+vars[0]+"}" {
		return "", false
	}
	return vars[0], true
}

// builtImage names a locally built image by its Dockerfile bytes and base
// reference, so a changed base digest changes the tool and generation hashes.
func builtImage(name string, source []byte, base string) string {
	full := generation.Hash(bytes.Join([][]byte{source, []byte(base)}, []byte{0}))
	return "olympus/" + name + ":sha256-" + full[len("sha256-"):len("sha256-")+12]
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
