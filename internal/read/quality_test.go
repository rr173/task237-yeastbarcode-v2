package read

import "testing"

func TestQualityPolicy(t *testing.T) {
	policy := NewQualityPolicy()
	if !policy.IsUsable([]int{20, 35, 40}) {
		t.Fatal("quality at the threshold should be usable")
	}
	if policy.IsUsable([]int{35, 19, 35}) {
		t.Fatal("one low score should exclude the read")
	}
	if got := policy.MeanQuality([]int{20, 30, 40}); got != 30 {
		t.Fatalf("mean quality=%v want 30", got)
	}
}
