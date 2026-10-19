package encode

import (
	"bytes"
	"math"
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

func TestIntegerRoundTrips(t *testing.T) {
	const large int64 = 9007199254740993
	for _, format := range []string{"yaml", "toml", "json"} {
		t.Run(format, func(t *testing.T) {
			input := map[string]any{"port": int64(8642), "large": large, "max": int64(math.MaxInt64), "min": int64(math.MinInt64), "nested": []any{map[string]any{"large": large}}}
			b, err := Marshal(format, input)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(b, []byte("9007199254740993")) || bytes.Contains(b, []byte("8642.0")) {
				t.Fatalf("integer changed: %s", b)
			}
			parsed, err := Unmarshal(format, b)
			if err != nil {
				t.Fatal(err)
			}
			if parsed["port"] != int64(8642) || parsed["large"] != large || parsed["max"] != int64(math.MaxInt64) || parsed["min"] != int64(math.MinInt64) {
				t.Fatalf("integers not preserved: %#v", parsed)
			}
			nested := parsed["nested"].([]any)[0].(map[string]any)
			if nested["large"] != large {
				t.Fatalf("nested integer lost precision: %#v", nested)
			}
			again, err := Marshal(format, parsed)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(b, again) {
				t.Fatalf("round trip changed output:\n%s\n%s", b, again)
			}
		})
	}
}
