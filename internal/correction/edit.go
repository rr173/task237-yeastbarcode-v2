// Package correction merges raw barcode reads into canonical clusters by
// collapsing sequencing errors (low edit distance) while preserving true
// mutations. It produces the corrected barcode that downstream lineage and
// discrimination logic consumes.
package correction

// EditDistance computes the Levenshtein distance between two barcode strings.
func EditDistance(a, b string) int {
	ra := []rune(a)
	rb := []rune(b)
	n, m := len(ra), len(rb)
	if n == 0 {
		return m
	}
	if m == 0 {
		return n
	}
	prev := make([]int, m+1)
	cur := make([]int, m+1)
	for j := 0; j <= m; j++ {
		prev[j] = j
	}
	for i := 1; i <= n; i++ {
		cur[0] = i
		for j := 1; j <= m; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min3(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[m]
}

// HammingDistance counts positions where two equal-length barcodes differ.
// Returns -1 if the lengths differ (not comparable under substitution model).
func HammingDistance(a, b string) int {
	ra := []rune(a)
	rb := []rune(b)
	if len(ra) != len(rb) {
		return -1
	}
	d := 0
	for i := range ra {
		if ra[i] != rb[i] {
			d++
		}
	}
	return d
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
