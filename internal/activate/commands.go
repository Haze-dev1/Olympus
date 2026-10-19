package activate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ToolStatus struct {
	Name       string `json:"name"`
	State      string `json:"state"`
	Health     string `json:"health"`
	Image      string `json:"image"`
	Generation int    `json:"generation"`
}

func (a *Activator) Status(ctx context.Context) ([]ToolStatus, error) {
	if _, err := os.Lstat(a.currentLink()); errors.Is(err, os.ErrNotExist) {
		return []ToolStatus{}, nil
	}
	t, n, ok, err := a.current()
	if err != nil {
		return nil, err
	}
	if !ok {
		return []ToolStatus{}, nil
	}
	cmd := exec.CommandContext(ctx, "docker", "compose", "-p", Project, "-f", t.composeFile(), "ps", "--all", "--format", "json")
	b, err := cmd.Output()
	if err != nil {
		return nil, errors.New("status: cannot read Compose state")
	}
	type row struct{ Service, State, Health, Image string }
	var rows []row
	if len(bytes.TrimSpace(b)) > 0 && bytes.TrimSpace(b)[0] == '[' {
		err = json.Unmarshal(b, &rows)
	} else {
		d := json.NewDecoder(bytes.NewReader(b))
		for {
			var r row
			e := d.Decode(&r)
			if errors.Is(e, io.EOF) {
				break
			}
			if e != nil {
				err = e
				break
			}
			rows = append(rows, r)
		}
	}
	if err != nil {
		return nil, errors.New("status: invalid Compose state")
	}
	names := []string{}
	for name := range t.manifest.Tools {
		names = append(names, name)
	}
	sort.Strings(names)
	result := []ToolStatus{}
	for _, name := range names {
		s := ToolStatus{Name: name, State: "missing", Health: "none", Image: t.manifest.Tools[name].Image, Generation: n}
		for _, r := range rows {
			if r.Service == name {
				s.State = r.State
				if r.Health != "" {
					s.Health = r.Health
				}
				s.Image = r.Image
			}
		}
		if s.State == "running" && a.Health != nil && a.Checker != nil {
			checks, e := a.Health.Health(t.id)
			if e != nil {
				return nil, e
			}
			if check, ok := checks[name]; ok {
				healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				e = a.Checker.Check(healthCtx, t.composeFile(), name, check)
				cancel()
				if e == nil {
					s.Health = "healthy"
				} else {
					s.Health = "unhealthy"
				}
			}
		}
		result = append(result, s)
	}
	return result, nil
}

// Destroy takes the same lock as activation and limits Docker deletion to the project.
func (a *Activator) Destroy(ctx context.Context, data bool) error {
	if err := a.ensureRoot(); err != nil {
		return err
	}
	lock, err := a.lock("destroy")
	if err != nil {
		return err
	}
	defer lock.Close()
	t, _, ok, err := a.current()
	if err != nil {
		return err
	}
	if ok {
		if data {
			cmd := exec.CommandContext(ctx, "docker", "compose", "-p", Project, "-f", t.composeFile(), "down", "--volumes", "--remove-orphans")
			if cmd.Run() != nil {
				return errors.New("destroy: Compose down failed")
			}
		} else if err := a.Runner.Down(ctx, t.composeFile()); err != nil {
			return err
		}
	}
	if data {
		// Include volumes of tools removed in older generations, but never touch
		// volumes outside the Olympus project.
		cmd := exec.CommandContext(ctx, "docker", "volume", "ls", "--filter", "label=com.docker.compose.project="+Project, "--format", "{{.Name}}")
		b, e := cmd.Output()
		if e != nil {
			return errors.New("destroy: cannot list project volumes")
		}
		names := strings.Fields(string(b))
		if len(names) > 0 {
			args := append([]string{"volume", "rm", "--"}, names...)
			if exec.CommandContext(ctx, "docker", args...).Run() != nil {
				return errors.New("destroy: cannot delete project volumes")
			}
		}
		if filepath.Clean(a.root()) == "/" {
			return fmt.Errorf("destroy: invalid root")
		}
		return os.RemoveAll(a.root())
	}
	return nil
}
