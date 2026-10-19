package cli

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Haze-dev1/Olympus/internal/deploy"
	"github.com/Haze-dev1/Olympus/internal/eval"
	"github.com/Haze-dev1/Olympus/internal/schema"
)

// Read values only at the command boundary, never during evaluation. Errors
// deliberately omit dotenv lines and values. There is no process-env fallback.
func toolSecrets(in eval.Inputs, file string) (map[string]deploy.ToolSecrets, error) {
	result := map[string]deploy.ToolSecrets{}
	needed := map[string]bool{}
	for _, t := range in.Stack.Tools() {
		for _, ref := range t.Secrets {
			for _, v := range schema.Variables(ref) {
				needed[v] = true
			}
		}
	}
	values := map[string]string{}
	if len(needed) > 0 {
		f, err := os.Open(filepath.Join(filepath.Dir(file), ".env"))
		if err != nil {
			return nil, errors.New("cannot read stack .env")
		}
		defer f.Close()
		scan := bufio.NewScanner(f)
		scan.Buffer(make([]byte, 4096), 4<<20)
		for scan.Scan() {
			line := strings.TrimSpace(scan.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			name, value, ok := strings.Cut(line, "=")
			name = strings.TrimSpace(name)
			if !ok || !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(name) {
				return nil, errors.New("invalid .env assignment")
			}
			if !needed[name] {
				continue
			}
			if _, exists := values[name]; exists {
				return nil, errors.New("duplicate .env assignment")
			}
			value = strings.TrimSpace(value)
			if strings.HasPrefix(value, "\"") {
				var e error
				value, e = strconv.Unquote(value)
				if e != nil {
					return nil, errors.New("invalid quoted .env value")
				}
			} else if strings.HasPrefix(value, "'") {
				if len(value) < 2 || !strings.HasSuffix(value, "'") {
					return nil, errors.New("invalid quoted .env value")
				}
				value = value[1 : len(value)-1]
			}
			if strings.ContainsAny(value, "\x00\r\n") {
				return nil, errors.New("unsupported multiline .env value")
			}
			values[name] = value
		}
		if scan.Err() != nil {
			return nil, errors.New("cannot read .env")
		}
	}
	for name, t := range in.Stack.Tools() {
		ts := deploy.ToolSecrets{Secrets: map[string]string{}, Environment: map[string]string{}}
		for secret, ref := range t.Secrets {
			if secret == "environment" || secret == "fingerprints.json" {
				return nil, errors.New("reserved secret name")
			}
			decl, ok := in.Recipes[t.Recipe].Secrets[secret]
			if !ok {
				return nil, errors.New("undeclared recipe secret")
			}
			vars := schema.Variables(ref)
			if len(vars) != 1 {
				return nil, errors.New("invalid secret reference")
			}
			v, ok := values[vars[0]]
			if !ok {
				return nil, errors.New("missing .env variable " + vars[0])
			}
			ts.Secrets[secret] = v
			if decl.Env != "" {
				if previous, ok := ts.Environment[decl.Env]; ok && previous != v {
					return nil, errors.New("conflicting secret environment mapping")
				}
				ts.Environment[decl.Env] = v
			}
		}
		result[name] = ts
	}
	return result, nil
}
