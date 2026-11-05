package recipe

import "testing"

func TestParseSize(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want uint64
	}{
		{"1", 1},
		{"512B", 512},
		{"4GB", 4_000_000_000},
		{"4 GiB", 4 << 30},
		{"512MiB", 512 << 20},
		{"2mb", 2_000_000},
		{"1T", 1_000_000_000_000},
	} {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseSize(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestParseSizeRejectsBadValues(t *testing.T) {
	for _, in := range []string{"", "4XB", "-1GB", "4.5GB", "GB", "999999999999999999999999GB"} {
		if _, err := ParseSize(in); err == nil {
			t.Fatalf("%q was accepted", in)
		}
	}
}
