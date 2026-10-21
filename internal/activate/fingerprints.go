package activate

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Fingerprints bind both declared secret values and their environment delivery
// to a server-specific key. Values and the key never enter a manifest or argv.
func Fingerprints(key []byte, secrets, environment map[string]string) map[string]string {
	result := map[string]string{}
	for kind, values := range map[string]map[string]string{"secret": secrets, "env": environment} {
		for name, value := range values {
			h := hmac.New(sha256.New, key)
			h.Write([]byte(kind + "\x00" + name + "\x00" + value))
			result[kind+":"+name] = hex.EncodeToString(h.Sum(nil))
		}
	}
	return result
}

func (a *Activator) serverKey() ([]byte, error) {
	p := filepath.Join(a.root(), "key")
	b, err := os.ReadFile(p)
	if err == nil {
		if len(b) != 32 {
			return nil, errors.New("activate: invalid server key")
		}
		return b, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, errors.New("activate: cannot read server key")
	}
	b = make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, errors.New("activate: cannot create server key")
	}
	_, err = f.Write(b)
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return nil, errors.New("activate: cannot write server key")
	}
	return b, nil
}

func readFingerprints(p string) (map[string]string, error) {
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	var m map[string]string
	if err != nil || json.Unmarshal(b, &m) != nil {
		return nil, errors.New("activate: cannot read secret fingerprints")
	}
	return m, nil
}
