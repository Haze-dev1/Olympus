package schema

import "testing"

func TestIntegerDataAndTypedDecode(t *testing.T) {
	d := ParseData("seed.yaml", []byte("port: 8642\nlarge: 9007199254740993\nmax: 9223372036854775807\nmin: -9223372036854775808\n"))
	if len(d.Errors) > 0 {
		t.Fatal(d.Errors)
	}
	values := d.Value.(map[string]any)
	if values["large"] != int64(9007199254740993) {
		t.Fatalf("precision lost before decoding: %#v", values["large"])
	}
	var result struct{ Port, Large, Max, Min int64 }
	if err := d.Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Port != 8642 || result.Large != 9007199254740993 || result.Max != 9223372036854775807 || result.Min != -9223372036854775808 {
		t.Fatalf("typed integers changed: %#v", result)
	}
}
