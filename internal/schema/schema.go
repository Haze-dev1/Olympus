// Package schema validates the published schema subset against positioned YAML.
// It never formats scalar input values into diagnostics.
package schema

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Haze-dev1/Olympus/schemas"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/token"
)

type Diagnostic struct {
	File         string
	Line, Column int
	Message      string
}

func (d Diagnostic) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s", d.File, d.Line, d.Column, d.Message)
}

type Document struct {
	File       string
	Value      any
	Nodes      map[string]ast.Node
	Errors     []Diagnostic
	References map[string][]string
}

func (d *Document) Add(path, message string) {
	n := d.Nodes[path]
	for n == nil && path != "" {
		i := strings.LastIndex(path, "/")
		path = path[:i]
		n = d.Nodes[path]
	}
	line, col := 1, 1
	if n != nil && n.GetToken() != nil {
		line = n.GetToken().Position.Line
		col = n.GetToken().Position.Column
	}
	d.Errors = append(d.Errors, Diagnostic{d.File, line, col, message})
}
func Child(path, key string) string {
	return path + "/" + strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}

var variable = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func Variables(s string) []string {
	var out []string
	for _, m := range variable.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	return out
}

func parseData(file string, data []byte) *Document {
	d := &Document{File: file, Nodes: map[string]ast.Node{}, References: map[string][]string{}}
	if len(data) > 4<<20 {
		d.Add("", "document exceeds 4 MiB")
		return d
	}
	tree, err := parser.ParseBytes(data, 0)
	if err != nil {
		line, col := 1, 1
		if e, ok := err.(interface{ GetToken() *token.Token }); ok && e.GetToken() != nil {
			line = e.GetToken().Position.Line
			col = e.GetToken().Position.Column
		}
		d.Errors = append(d.Errors, Diagnostic{file, line, col, "invalid YAML (check syntax and duplicate keys)"})
		return d
	}
	if len(tree.Docs) != 1 || tree.Docs[0].Body == nil {
		d.Add("", "expected one YAML document")
		return d
	}
	d.Value = d.read(tree.Docs[0].Body, "", 0)
	if len(d.Errors) > 0 {
		return d
	}
	return d
}

// ParseData accepts only bounded, inert YAML data, retaining source positions.
func ParseData(file string, data []byte) *Document { return parseData(file, data) }

func Parse(file string, data []byte, kind string) *Document {
	d := parseData(file, data)
	if len(d.Errors) > 0 {
		return d
	}
	b, err := schemas.Files.ReadFile(kind + ".schema.json")
	if err != nil {
		panic(err)
	}
	var rule map[string]any
	if err = json.Unmarshal(b, &rule); err != nil {
		panic(err)
	}
	d.validate(d.Value, rule, "")
	return d
}
func (d *Document) read(n ast.Node, path string, depth int) any {
	d.Nodes[path] = n
	if depth > 64 {
		d.Add(path, "YAML nesting exceeds 64 levels")
		return nil
	}
	switch n := n.(type) {
	case *ast.MappingNode:
		m := map[string]any{}
		for _, p := range n.Values {
			k, ok := p.Key.(*ast.StringNode)
			if !ok {
				d.Add(path, "mapping keys must be strings")
				continue
			}
			next := Child(path, k.Value)
			if strings.Contains(k.Value, "${") {
				d.Nodes[next] = k
				d.Add(next, "variables are forbidden in keys")
			}
			if _, ok := m[k.Value]; ok {
				d.Add(next, "duplicate key")
				continue
			}
			m[k.Value] = d.read(p.Value, next, depth+1)
		}
		return m
	case *ast.SequenceNode:
		a := make([]any, 0, len(n.Values))
		for i, v := range n.Values {
			a = append(a, d.read(v, Child(path, strconv.Itoa(i)), depth+1))
		}
		return a
	case *ast.StringNode:
		return n.Value
	case *ast.LiteralNode:
		return n.Value.Value
	case *ast.BoolNode:
		return n.Value
	case *ast.IntegerNode:
		b, _ := json.Marshal(n.Value)
		var v float64
		if json.Unmarshal(b, &v) != nil {
			d.Add(path, "integer out of range")
		}
		return v
	case *ast.FloatNode:
		return n.Value
	case *ast.NullNode:
		return nil
	default:
		d.Add(path, "unsupported YAML node (aliases, anchors and tags are forbidden)")
		return nil
	}
}
func (d *Document) validate(v any, r map[string]any, path string) {
	if alternatives, ok := r["anyOf"].([]any); ok {
		for _, a := range alternatives {
			before := len(d.Errors)
			d.validate(v, a.(map[string]any), path)
			if len(d.Errors) == before {
				return
			}
			d.Errors = d.Errors[:before]
		}
		d.Add(path, "expected 'all' or a list of paths")
		return
	}
	typ, _ := r["type"].(string)
	valid := false
	switch typ {
	case "object":
		_, valid = v.(map[string]any)
	case "array":
		_, valid = v.([]any)
	case "string":
		_, valid = v.(string)
	case "integer":
		f, ok := v.(float64)
		valid = ok && math.Trunc(f) == f
	}
	if !valid {
		d.Add(path, "expected "+typ)
		return
	}
	if enum, ok := r["enum"].([]any); ok {
		found := false
		for _, e := range enum {
			if e == v {
				found = true
			}
		}
		if !found {
			d.Add(path, "value is not an allowed choice")
		}
	}
	switch v := v.(type) {
	case map[string]any:
		props, _ := r["properties"].(map[string]any)
		if required, ok := r["required"].([]any); ok {
			for _, k := range required {
				if _, ok := v[k.(string)]; !ok {
					d.Add(path, "missing required field "+k.(string))
				}
			}
		}
		keys := Keys(v)
		for _, k := range keys {
			p, ok := props[k]
			if !ok {
				p, ok = r["additionalProperties"].(map[string]any)
			}
			if ok {
				d.validate(v[k], p.(map[string]any), Child(path, k))
			} else {
				d.Add(Child(path, k), "unknown field")
			}
		}
	case []any:
		if min, ok := r["minItems"].(float64); ok && len(v) < int(min) {
			d.Add(path, "list must not be empty")
		}
		seen := map[string]bool{}
		for i, item := range v {
			p := Child(path, strconv.Itoa(i))
			b, _ := json.Marshal(item)
			if r["uniqueItems"] == true && seen[string(b)] {
				d.Add(p, "duplicate list item")
			}
			seen[string(b)] = true
			d.validate(item, r["items"].(map[string]any), p)
		}
	case string:
		if n, ok := d.Nodes[path].(*ast.StringNode); ok && n.Token.Type == token.StringType && regexp.MustCompile(`(?i)^(no|on|off|yes|0[0-9]+)$`).MatchString(v) {
			d.Add(path, "expected string with quotes around YAML-ambiguous scalar")
		}
		if min, ok := r["minLength"].(float64); ok && len(v) < int(min) {
			d.Add(path, "string must not be empty")
		}
		if pattern, ok := r["pattern"].(string); ok && !regexp.MustCompile(pattern).MatchString(v) {
			d.Add(path, "string does not match required pattern")
		}
		if strings.Contains(v, "${") {
			if r["x-secret"] != true {
				d.Add(path, "variables are allowed only in secret-capable fields")
				return
			}
			rest := variable.ReplaceAllString(v, "")
			if strings.Contains(rest, "${") {
				d.Add(path, "invalid variable reference")
			}
			for _, name := range Variables(v) {
				d.References[name] = append(d.References[name], path)
			}
		}
	case float64:
		if min, ok := r["minimum"].(float64); ok && v < min {
			d.Add(path, "integer below minimum")
		}
		if max, ok := r["maximum"].(float64); ok && v > max {
			d.Add(path, "integer above maximum")
		}
	}
}
func Keys[V any](m map[string]V) []string {
	a := make([]string, 0, len(m))
	for k := range m {
		a = append(a, k)
	}
	sort.Strings(a)
	return a
}
func (d *Document) Decode(out any) error {
	b, err := json.Marshal(d.Value)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}
