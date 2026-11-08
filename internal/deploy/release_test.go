package deploy

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Haze-dev1/Olympus/internal/eval"
)

func dummyELF64x86() []byte {
	hdr := make([]byte, 64)
	copy(hdr[0:4], []byte{0x7f, 'E', 'L', 'F'})
	hdr[4] = 2     // 64-bit
	hdr[5] = 1     // little-endian
	hdr[6] = 1     // version
	hdr[16] = 2    // ET_EXEC
	hdr[18] = 0x3e // EM_X86_64
	hdr[20] = 1    // version
	return hdr
}

func TestUploadActivatorReleaseChecksumVerification(t *testing.T) {
	elfData := dummyELF64x86()
	expectedHash := fmt.Sprintf("%x", sha256.Sum256(elfData))
	wrongHash := strings.Repeat("0", 64)

	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "olym-linux-amd64")
	if err := os.WriteFile(binPath, elfData, 0755); err != nil {
		t.Fatal(err)
	}

	makeConn := func() *fakeConn {
		var uploaded []byte
		return &fakeConn{run: func(a []string, in io.Reader) ([]byte, []byte, error) {
			switch a[0] {
			case "uname":
				return okOutput("x86_64")
			case "install":
				uploaded, _ = io.ReadAll(in)
			case "sha256sum":
				return okOutput(fmt.Sprintf("%x  binary", sha256.Sum256(uploaded)))
			}
			if strings.HasPrefix(a[0], root) {
				return okOutput("0.1.0-dev")
			}
			return okOutput("")
		}}
	}

	t.Run("matching release checksum succeeds", func(t *testing.T) {
		f := makeConn()
		c := Client{
			Conn:             f,
			Version:          "0.1.0-dev",
			ReleaseChecksums: map[string]string{"x86_64": expectedHash},
		}
		if err := c.UploadActivator(context.Background(), binPath); err != nil {
			t.Fatalf("expected success, got %v", err)
		}
	})

	t.Run("mismatched release checksum fails closed", func(t *testing.T) {
		f := makeConn()
		c := Client{
			Conn:             f,
			Version:          "0.1.0-dev",
			ReleaseChecksums: map[string]string{"x86_64": wrongHash},
		}
		err := c.UploadActivator(context.Background(), binPath)
		if err == nil {
			t.Fatal("expected failure on mismatched release checksum, got nil")
		}
		if !strings.Contains(err.Error(), "activator checksum does not match release build") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("empty release checksum requires dev opt-in", func(t *testing.T) {
		f := makeConn()
		c := Client{
			Conn:    f,
			Version: eval.DevelopmentVersion,
		}
		err := c.UploadActivator(context.Background(), binPath)
		if err == nil || !strings.Contains(err.Error(), "-unsafe-dev-activator") {
			t.Fatalf("expected dev opt-in error, got %v", err)
		}
		c = Client{
			Conn:               makeConn(),
			Version:            eval.DevelopmentVersion,
			UnsafeDevActivator: true,
		}
		if err := c.UploadActivator(context.Background(), binPath); err != nil {
			t.Fatalf("expected opt-in dev build to succeed, got %v", err)
		}
	})
}

func TestVerifyActivatorReleaseChecksum(t *testing.T) {
	expectedHash := strings.Repeat("a", 64)

	makeConn := func(remoteHash string) *fakeConn {
		return &fakeConn{run: func(a []string, in io.Reader) ([]byte, []byte, error) {
			switch a[0] {
			case "uname":
				return okOutput("x86_64")
			case "sha256sum":
				return okOutput(fmt.Sprintf("%s  binary", remoteHash))
			}
			if strings.HasPrefix(a[0], root) {
				return okOutput("1.0.0")
			}
			return okOutput("")
		}}
	}

	t.Run("remote matches release checksum without local binary", func(t *testing.T) {
		f := makeConn(expectedHash)
		c := Client{
			Conn:             f,
			Version:          "1.0.0",
			ReleaseChecksums: map[string]string{"x86_64": expectedHash},
		}
		if err := c.VerifyActivator(context.Background(), ""); err != nil {
			t.Fatalf("expected verify success, got %v", err)
		}
	})

	t.Run("remote mismatch fails", func(t *testing.T) {
		f := makeConn(strings.Repeat("b", 64))
		c := Client{
			Conn:             f,
			Version:          "1.0.0",
			ReleaseChecksums: map[string]string{"x86_64": expectedHash},
		}
		err := c.VerifyActivator(context.Background(), "")
		if err == nil {
			t.Fatal("expected verify failure, got nil")
		}
		if !strings.Contains(err.Error(), "checksum mismatch") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// A development build has no embedded release checksum, so it cannot prove the
// activator matches a release. Uploading or running it must require the explicit
// -unsafe-dev-activator opt-in (security finding F3).
func TestDevelopmentActivatorRequiresExplicitOptIn(t *testing.T) {
	elfData := dummyELF64x86()
	checksum := fmt.Sprintf("%x", sha256.Sum256(elfData))
	binPath := filepath.Join(t.TempDir(), "olym-linux-amd64")
	if err := os.WriteFile(binPath, elfData, 0755); err != nil {
		t.Fatal(err)
	}
	conn := func(hash string) *fakeConn {
		return &fakeConn{run: func(a []string, in io.Reader) ([]byte, []byte, error) {
			switch a[0] {
			case "uname":
				return okOutput("x86_64")
			case "install":
				_, _ = io.Copy(io.Discard, in)
			case "sha256sum":
				return okOutput(hash + "  binary")
			}
			if strings.HasPrefix(a[0], root) {
				return okOutput(eval.DevelopmentVersion)
			}
			return okOutput("")
		}}
	}

	t.Run("upload refuses without opt-in", func(t *testing.T) {
		c := Client{Conn: conn(checksum), Version: eval.DevelopmentVersion}
		err := c.UploadActivator(context.Background(), binPath)
		if err == nil {
			t.Fatal("development build uploaded an activator with no release checksum without -unsafe-dev-activator")
		}
		if !strings.Contains(err.Error(), "-unsafe-dev-activator") {
			t.Fatalf("error does not name the opt-in flag: %v", err)
		}
	})

	t.Run("verify refuses without opt-in", func(t *testing.T) {
		c := Client{Conn: conn(checksum), Version: eval.DevelopmentVersion}
		err := c.VerifyActivator(context.Background(), binPath)
		if err == nil {
			t.Fatal("development build ran an activator with no release checksum without -unsafe-dev-activator")
		}
		if !strings.Contains(err.Error(), "-unsafe-dev-activator") {
			t.Fatalf("error does not name the opt-in flag: %v", err)
		}
	})

	t.Run("upload and verify succeed with opt-in", func(t *testing.T) {
		c := Client{Conn: conn(checksum), Version: eval.DevelopmentVersion, UnsafeDevActivator: true}
		if err := c.UploadActivator(context.Background(), binPath); err != nil {
			t.Fatalf("opt-in upload: %v", err)
		}
		c = Client{Conn: conn(checksum), Version: eval.DevelopmentVersion, UnsafeDevActivator: true}
		if err := c.VerifyActivator(context.Background(), binPath); err != nil {
			t.Fatalf("opt-in verify: %v", err)
		}
	})
}

func TestExpectedChecksum(t *testing.T) {
	origAmd64 := ReleaseChecksumLinuxAmd64
	origArm64 := ReleaseChecksumLinuxArm64
	defer func() {
		ReleaseChecksumLinuxAmd64 = origAmd64
		ReleaseChecksumLinuxArm64 = origArm64
	}()

	ReleaseChecksumLinuxAmd64 = "hash-amd64"
	ReleaseChecksumLinuxArm64 = "hash-arm64"

	if got := ExpectedChecksum("x86_64"); got != "hash-amd64" {
		t.Fatalf("got %q, want hash-amd64", got)
	}
	if got := ExpectedChecksum("amd64"); got != "hash-amd64" {
		t.Fatalf("got %q, want hash-amd64", got)
	}
	if got := ExpectedChecksum("aarch64"); got != "hash-arm64" {
		t.Fatalf("got %q, want hash-arm64", got)
	}
	if got := ExpectedChecksum("arm64"); got != "hash-arm64" {
		t.Fatalf("got %q, want hash-arm64", got)
	}
	if got := ExpectedChecksum("unknown"); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestMissingReleaseChecksumFailsClosed(t *testing.T) {
	c := Client{Version: "1.0.0", Conn: &fakeConn{run: func(a []string, _ io.Reader) ([]byte, []byte, error) { return okOutput("x86_64") }}}
	if err := c.UploadActivator(context.Background(), ""); err == nil {
		t.Fatal("upload accepted missing checksum")
	}
	if err := c.VerifyActivator(context.Background(), ""); err == nil {
		t.Fatal("verify accepted missing checksum")
	}
	if err := CheckReleaseBuild("1.0.0"); err == nil {
		t.Fatal("release accepted missing checksums")
	}
	if err := CheckReleaseBuild("0.1.0-dev"); err != nil {
		t.Fatal(err)
	}
}

func TestBundledActivatorSelectionAndChecksum(t *testing.T) {
	path, err := bundledActivator("x86_64")
	if err != nil || filepath.Base(path) != "olym-activator_linux_amd64" {
		t.Fatalf("unexpected bundle path %q: %v", path, err)
	}
	path, err = bundledActivator("aarch64")
	if err != nil || filepath.Base(path) != "olym-activator_linux_arm64" {
		t.Fatalf("unexpected bundle path %q: %v", path, err)
	}
	if _, err = bundledActivator("unknown"); err == nil {
		t.Fatal("accepted unsupported architecture")
	}
}

// Run with a test executable next to release artifacts and their embedded checksum.
func TestReleaseBundle(t *testing.T) {
	if os.Getenv("OLYM_TEST_RELEASE_BUNDLE") != "1" {
		t.Skip("requires built release bundle")
	}
	var uploaded []byte
	c := Client{Version: "9.9.9", Conn: &fakeConn{run: func(a []string, in io.Reader) ([]byte, []byte, error) {
		switch a[0] {
		case "uname":
			return okOutput("x86_64")
		case "install":
			uploaded, _ = io.ReadAll(in)
		case "sha256sum":
			return okOutput(fmt.Sprintf("%x  binary", sha256.Sum256(uploaded)))
		}
		if strings.HasPrefix(a[0], root) {
			return okOutput("9.9.9")
		}
		return okOutput("")
	}}}
	if err := c.UploadActivator(context.Background(), ""); err != nil {
		t.Fatalf("bundled activator rejected: %v", err)
	}
	b := append([]byte(nil), uploaded...)
	b[4096] ^= 1
	modified := filepath.Join(t.TempDir(), "modified")
	if err := os.WriteFile(modified, b, 0755); err != nil {
		t.Fatal(err)
	}
	if err := c.UploadActivator(context.Background(), modified); err == nil || !strings.Contains(err.Error(), "checksum does not match") {
		t.Fatalf("modified activator accepted: %v", err)
	}
	t.Log("bundled activator accepted; modified activator rejected")
}
