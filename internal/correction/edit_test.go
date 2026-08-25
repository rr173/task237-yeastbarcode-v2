package correction

import "testing"

func TestEditDistance(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"ACGT", "ACGT", 0},
		{"ACGT", "ACGA", 1},
		{"ACGT", "AGT", 1},   // deletion
		{"ACGT", "ACGGT", 1}, // insertion
		{"", "ACGT", 4},
	}
	for _, c := range cases {
		if got := EditDistance(c.a, c.b); got != c.want {
			t.Errorf("EditDistance(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestHammingDistance(t *testing.T) {
	if d := HammingDistance("ACGTACGT", "ACGTACGA"); d != 1 {
		t.Errorf("HammingDistance=1 got %d", d)
	}
	if d := HammingDistance("ACGT", "AC"); d != -1 {
		t.Errorf("HammingDistance unequal len should be -1, got %d", d)
	}
}
