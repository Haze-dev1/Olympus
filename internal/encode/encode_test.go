package encode

import (
	"bytes"
	"testing"
)

func TestDeterministicStructuredEncoding(t *testing.T) {
	for _, format := range []string{"yaml", "toml", "json"} {
		a := map[string]any{"z": "colon: # quote \" newline\n", "a": map[string]any{"b": []any{"no", "08", "on"}, "a": "first"}}
		b := map[string]any{"a": map[string]any{"a": "first", "b": []any{"no", "08", "on"}}, "z": "colon: # quote \" newline\n"}
		first, e := Marshal(format, a)
		if e != nil {
			t.Fatal(e)
		}
		second, e := Marshal(format, b)
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("%s output depends on map order", format)
		}
		doc, e := Unmarshal(format, first)
		if e != nil {
			t.Fatal(e)
		}
		if doc["z"] != a["z"] {
			t.Fatalf("%s changed string", format)
		}
	}
}

func TestUnsafeStructuredSeedsRejected(t *testing.T) {
	for _, input := range []string{"base: &secret {a: b}\ncopy: *secret\n", "base: !!python/object {}\n", "one: 1\n---\ntwo: 2\n"} {
		if _, e := Unmarshal("yaml", []byte(input)); e == nil {
			t.Fatal("unsafe structured YAML accepted")
		}
	}
	if _, e := Unmarshal("json", []byte(`{} {"extra":true}`)); e == nil {
		t.Fatal("trailing JSON document accepted")
	}
}
