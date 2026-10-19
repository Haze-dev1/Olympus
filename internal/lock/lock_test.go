package lock

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testRev  = "0123456789abcdef0123456789abcdef01234567"
	testHash = "sha256-1111111111111111111111111111111111111111111111111111111111111111"
	testImg  = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
)

func testFile() *File {
	return &File{
		Version: 1,
		Inputs: map[string]Input{
			"recipes": {
				URL:  "github:haze/olympus/recipes",
				Rev:  testRev,
				Hash: testHash,
			},
		},
		Tools: map[string]Tool{
			"hermes": {Image: "nousresearch/hermes-agent@" + testImg},
			"codex":  {Image: "ghcr.io/acme/codex@sha256:3333333333333333333333333333333333333333333333333333333333333333"},
		},
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "stack.lock")
	if err := Write(first, testFile()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}

	want := `{
  "version": 1,
  "inputs": {
    "recipes": {
      "url": "github:haze/olympus/recipes",
      "rev": "0123456789abcdef0123456789abcdef01234567",
      "hash": "sha256-1111111111111111111111111111111111111111111111111111111111111111"
    }
  },
  "tools": {
    "codex": {
      "image": "ghcr.io/acme/codex@sha256:3333333333333333333333333333333333333333333333333333333333333333"
    },
    "hermes": {
      "image": "nousresearch/hermes-agent@sha256:2222222222222222222222222222222222222222222222222222222222222222"
    }
  }
}
`
	if string(data) != want {
		t.Fatalf("unexpected encoding:\n%s\nwant:\n%s", data, want)
	}

	read, err := Read(first)
	if err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(dir, "second.lock")
	if err := Write(second, read); err != nil {
		t.Fatal(err)
	}
	again, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(data) {
		t.Fatalf("round trip is not byte-identical:\n%s\n%s", data, again)
	}
}

func TestReadRejectsInvalidLock(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "unknown field",
			body: `{"version":1,"inputs":{"recipes":{"url":"github:haze/olympus/recipes","rev":"` + testRev + `","hash":"` + testHash + `"}},"tools":{},"extra":1}`,
			want: "unknown field",
		},
		{
			name: "unpinned image",
			body: `{"version":1,"inputs":{"recipes":{"url":"u","rev":"` + testRev + `","hash":"` + testHash + `"}},"tools":{"hermes":{"image":"nousresearch/hermes-agent:latest"}}}`,
			want: "not pinned",
		},
		{
			name: "bad hash",
			body: `{"version":1,"inputs":{"recipes":{"url":"u","rev":"` + testRev + `","hash":"md5-abc"}},"tools":{}}`,
			want: "invalid hash",
		},
		{
			name: "trailing data",
			body: `{"version":1,"inputs":{"recipes":{"url":"u","rev":"` + testRev + `","hash":"` + testHash + `"}},"tools":{}}{}`,
			want: "trailing data",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "stack.lock")
			if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := Read(path)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got error %v, want one containing %q", err, tc.want)
			}
		})
	}
}
