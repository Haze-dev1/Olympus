// Package recipe loads and confines local declarative recipes.
package recipe

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Haze-dev1/Olympus/internal/schema"
	"github.com/Haze-dev1/Olympus/internal/stack"
)

type Secret struct{ Env string }
type Managed struct{ File, Format, Style, Key string }
type Skills struct{ Dir, Mode string }
type Channel struct{ Env map[string]string }
type Build struct{ Src, Dst string }
type Template struct{ Src, Dst string }
type Recipe struct {
	Name, Kind, Image, User string
	Resources               struct {
		MinRAM string `json:"min_ram"`
	}
	Volumes  map[string]string
	Ports    []int
	Command  []string
	Writable []string
	Build    *Build
	Env      map[string]string
	Secrets  map[string]Secret
	Supports struct {
		MCPs     *Managed
		Skills   *Skills
		Channels map[string]Channel
	}
	Templates []Template
	Hooks     struct {
		PostInstall []string `json:"post_install"`
	}
	Health struct {
		HTTP *struct {
			Port int
			Path string
		}
		Command []string
		Timeout string
	}
	Dir string `json:"-"`
}

func Load(root, name string) (*Recipe, *schema.Document) {
	file := filepath.Join(root, name, "recipe.yaml")
	d := &schema.Document{File: file}
	if !stack.ValidName(name) {
		d.Add("", "invalid recipe name")
		return nil, d
	}
	b, err := ReadFile(root, filepath.Join(name, "recipe.yaml"))
	if err != nil {
		message := "recipe not found or unsafe path"
		entries, _ := os.ReadDir(root)
		best := ""
		distance := 100
		for _, e := range entries {
			if e.IsDir() && stack.ValidName(e.Name()) {
				n := editDistance(name, e.Name())
				if n < distance {
					distance = n
					best = e.Name()
				}
			}
		}
		if best != "" {
			message += "; closest recipe: " + best
		}
		d.Add("", message)
		return nil, d
	}
	r, d := Parse(file, b)
	if r != nil {
		r.Dir = filepath.Join(root, name)
		if r.Name != name {
			d.Add("/name", "recipe name must match its folder")
		}
	}
	return r, d
}
func Parse(file string, b []byte) (*Recipe, *schema.Document) {
	d := schema.Parse(file, b, "recipe")
	if len(d.Errors) > 0 {
		return nil, d
	}
	var r Recipe
	if d.Decode(&r) != nil {
		d.Add("", "cannot decode recipe")
		return nil, d
	}
	for _, name := range schema.Keys(r.Volumes) {
		p := r.Volumes[name]
		if !stack.ValidName(name) || !CleanAbsolute(p) || p == "/" || p == "/run/secrets" || strings.HasPrefix(p, "/run/secrets/") {
			d.Add(schema.Child("/volumes", name), "invalid volume name or container path")
		}
	}
	for _, name := range schema.Keys(r.Volumes) {
		for _, other := range schema.Keys(r.Volumes) {
			if name < other && (inside(r.Volumes[name], r.Volumes[other]) || inside(r.Volumes[other], r.Volumes[name])) {
				d.Add("/volumes", "volume paths must not overlap")
			}
		}
	}
	destinations := map[string]bool{}
	for i, t := range r.Templates {
		p := fmt.Sprintf("/templates/%d", i)
		if !SafeRelative(t.Src) {
			d.Add(p+"/src", "template source must be a confined relative path")
		}
		if !r.Confined(t.Dst) {
			d.Add(p+"/dst", "template destination must be inside a tool volume")
		}
		if destinations[t.Dst] {
			d.Add(p+"/dst", "duplicate template destination")
		}
		destinations[t.Dst] = true
	}
	if m := r.Supports.MCPs; m != nil {
		if !r.Confined(m.File) {
			d.Add("/supports/mcps/file", "managed file must be inside a tool volume")
		}
		for _, part := range strings.Split(m.Key, ".") {
			if part == "" {
				d.Add("/supports/mcps/key", "managed key must contain nonempty dot-separated components")
			}
		}
	}
	if b := r.Build; b != nil {
		if !SafeRelative(b.Src) {
			d.Add("/build/src", "build source must be a confined relative path")
		}
		if !r.Confined(b.Dst) {
			d.Add("/build/dst", "build destination must be inside a tool volume")
		}
	}
	for i, p := range r.Writable {
		if !r.Confined(p) || !ShellSafe(p) {
			d.Add(fmt.Sprintf("/writable/%d", i), "writable path must be inside a tool volume")
		}
	}
	for k, c := range r.Supports.Channels {
		if c.Env["token"] == "" {
			d.Add(schema.Child("/supports/channels", k)+"/env", "channel requires a token environment variable")
		}
	}
	if r.Supports.Skills != nil {
		if !r.Confined(r.Supports.Skills.Dir) {
			d.Add("/supports/skills/dir", "skills directory must be inside a tool volume")
		}
		if r.Supports.Skills.Mode != "copy" && r.Supports.Skills.Mode != "mount" {
			d.Add("/supports/skills/mode", "skills mode must be copy or mount")
		}
	}
	if r.Health.Timeout != "" {
		t, e := time.ParseDuration(r.Health.Timeout)
		if e != nil || t <= 0 {
			d.Add("/health/timeout", "expected positive duration")
		}
	}
	if r.Health.HTTP != nil && len(r.Health.Command) > 0 {
		d.Add("/health", "choose HTTP or command health check")
	}
	if !validImage(r.Image) {
		d.Add("/image", "image must have an explicit tag or sha256 digest")
	}
	envs := map[string]bool{}
	for _, name := range schema.Keys(r.Secrets) {
		if _, exists := r.Env[r.Secrets[name].Env]; exists {
			d.Add(schema.Child("/secrets", name), "secret environment target conflicts with recipe env")
		}
		if envs[r.Secrets[name].Env] {
			d.Add(schema.Child("/secrets", name), "secret environment targets must be unique")
		}
		envs[r.Secrets[name].Env] = true
		if !stack.ValidName(name) {
			d.Add(schema.Child("/secrets", name), "invalid secret name")
		}
	}
	return &r, d
}
func CleanAbsolute(p string) bool {
	return strings.HasPrefix(p, "/") && !strings.Contains(p, "\\") && path.Clean(p) == p && !hasDotDot(p)
}
func SafeRelative(p string) bool {
	return p != "" && filepath.Clean(p) != "." && !filepath.IsAbs(p) && !strings.Contains(p, "\\") && !hasDotDot(p)
}

// ShellSafe reports whether a container path is safe to interpolate into a
// single-quoted shell command without escaping (A2/A8).
func ShellSafe(p string) bool { return shellSafeRe.MatchString(p) }

var shellSafeRe = regexp.MustCompile(`^/[A-Za-z0-9._@+-]+(/[A-Za-z0-9._@+-]+)*$`)

func hasDotDot(p string) bool {
	for _, c := range strings.Split(p, "/") {
		if c == ".." {
			return true
		}
	}
	return false
}
func inside(p, root string) bool { return p == root || strings.HasPrefix(p, root+"/") }
func (r *Recipe) Confined(p string) bool {
	if !CleanAbsolute(p) {
		return false
	}
	for _, root := range r.Volumes {
		if CleanAbsolute(root) && root != "/" && p != root && inside(p, root) {
			return true
		}
	}
	return false
}

// ReadFile rejects symlinks, including symlink ancestors, then uses os.Root to
// enforce confinement even if a path changes between validation and opening.
func ReadFile(root, rel string) ([]byte, error) {
	if !SafeRelative(rel) {
		return nil, fmt.Errorf("unsafe relative path")
	}
	current := root
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("unsafe root")
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if strings.HasPrefix(part, ".env") || part == "id_rsa" || part == "id_ed25519" || strings.HasSuffix(part, ".pem") || strings.HasSuffix(part, ".key") {
			return nil, fmt.Errorf("sensitive source file is forbidden")
		}
		current = filepath.Join(current, part)
		info, err = os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("unsafe path")
		}
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("expected regular file")
	}
	if info.Size() > 4<<20 {
		return nil, fmt.Errorf("file exceeds 4 MiB")
	}
	dir, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	return dir.ReadFile(rel)
}
func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		row := make([]int, len(b)+1)
		row[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			row[j] = min(row[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev = row
	}
	return prev[len(b)]
}

func validImage(image string) bool {
	if strings.ContainsAny(image, " \t\r\n") {
		return false
	}
	if strings.Contains(image, "@") {
		parts := strings.Split(image, "@sha256:")
		return len(parts) == 2 && parts[0] != "" && regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(parts[1])
	}
	colon := strings.LastIndex(image, ":")
	return colon > strings.LastIndex(image, "/") && colon > 0 && regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`).MatchString(image[colon+1:])
}
