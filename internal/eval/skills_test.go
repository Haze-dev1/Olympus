package eval

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Haze-dev1/Olympus/internal/encode"
)

func TestCopiedSkillsPruned(t *testing.T) {
	in := realInputs(t)
	root := t.TempDir()
	skillRoot := filepath.Join(root, "skills")
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(skillRoot, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	run := func(wantError bool) {
		t.Helper()
		g, err := Build(in)
		if err != nil {
			t.Fatal(err)
		}
		var c Compose
		doc, err := encode.Unmarshal("yaml", g.Files["compose.yaml"])
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &c); err != nil {
			t.Fatal(err)
		}
		init, ok := c.Services["hermes-init"]
		if !ok {
			t.Fatal("missing init with no selected skills")
		}
		script := strings.ReplaceAll(init.Entrypoint[2], "$$", "$")
		script = strings.ReplaceAll(script, "/opt/data/skills", skillRoot)
		for _, m := range init.Volumes {
			if m.Type != "bind" {
				continue
			}
			rel := strings.TrimPrefix(m.Source, "/var/lib/olympus/")
			source := filepath.Join(root, strings.TrimPrefix(m.Target, "/"))
			if err := os.MkdirAll(filepath.Dir(source), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(source, g.Files[rel], 0600); err != nil {
				t.Fatal(err)
			}
			script = strings.ReplaceAll(script, m.Target, source)
		}
		// Confine the writable config copy to the same disposable filesystem.
		script = strings.ReplaceAll(script, "/opt/data/config.yaml", filepath.Join(root, "config.yaml"))
		out, err := exec.Command("/bin/sh", "-c", script).CombinedOutput()
		if (err != nil) != wantError {
			t.Fatalf("init: %v %s", err, out)
		}
	}
	write("personal/SKILL.md", "keep")
	run(false)
	copied := "deploy-checklist/SKILL.md"
	if _, err := os.Stat(filepath.Join(skillRoot, copied)); err != nil {
		t.Fatal(err)
	}
	write("deploy-checklist/custom.md", "keep")
	// Remove a file from a retained skill before removing the entire selection.
	for key := range in.Skills {
		in.Skills[key]["obsolete.md"] = []byte("old")
	}
	run(false)
	for key := range in.Skills {
		delete(in.Skills[key], "obsolete.md")
	}
	run(false)
	if _, err := os.Stat(filepath.Join(skillRoot, "deploy-checklist/obsolete.md")); !os.IsNotExist(err) {
		t.Fatalf("obsolete file survived: %v", err)
	}
	entry := in.Stack.Agents["hermes"]
	entry.Skills = nil
	in.Stack.Agents["hermes"] = entry
	run(false)
	run(false)
	if _, err := os.Stat(filepath.Join(skillRoot, copied)); !os.IsNotExist(err) {
		t.Fatalf("removed skill survived: %v", err)
	}
	for _, rel := range []string{"personal/SKILL.md", "deploy-checklist/custom.md"} {
		b, err := os.ReadFile(filepath.Join(skillRoot, rel))
		if err != nil || string(b) != "keep" {
			t.Fatalf("unowned file changed: %s %v", rel, err)
		}
	}
	entry.Skills = "all"
	in.Stack.Agents["hermes"] = entry
	run(false)
	if _, err := os.Stat(filepath.Join(skillRoot, copied)); err != nil {
		t.Fatal("rollback did not restore skill", err)
	}
	// A writable ledger cannot authorize traversal or symlink escapes.
	outside := filepath.Join(root, "outside")
	if err := os.WriteFile(outside, []byte("safe"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"../outside\n", "/outside\n", "personal/SKILL.md\n../outside\n", "personal/./SKILL.md\n"} {
		write(skillLedger, bad)
		run(true)
	}
	if err := os.Symlink(root, filepath.Join(skillRoot, "escape")); err != nil {
		t.Fatal(err)
	}
	write(skillLedger, "escape/outside\n")
	run(true)
	if b, err := os.ReadFile(filepath.Join(skillRoot, "personal/SKILL.md")); err != nil || string(b) != "keep" {
		t.Fatal("invalid ledger caused partial pruning")
	}
	b, err := os.ReadFile(outside)
	if err != nil || string(b) != "safe" {
		t.Fatal("pruning escaped skill directory")
	}
}
