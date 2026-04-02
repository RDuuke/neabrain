package version

import "testing"

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.2.0", "0.2.0", 0},
		{"0.2.0", "0.3.0", -1},
		{"0.3.0", "0.2.0", 1},
		{"1.0.0", "0.9.9", 1},
		{"0.2.1", "0.2.0", 1},
	}
	for _, c := range cases {
		got := compare(c.a, c.b)
		if got != c.want {
			t.Errorf("compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestParseSemver(t *testing.T) {
	got := parseSemver("1.2.3")
	if got != [3]int{1, 2, 3} {
		t.Fatalf("unexpected: %v", got)
	}
}
