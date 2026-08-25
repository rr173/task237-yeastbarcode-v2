package read

// QualityPolicy decides whether a read is usable for correction. A read with
// any quality score below MinPhred is treated as low-quality and excluded from
// clustering so that sequencing errors are not mistaken for mutations.
type QualityPolicy struct {
	MinPhred int
}

// NewQualityPolicy builds the default policy (phred >= 20).
func NewQualityPolicy() QualityPolicy { return QualityPolicy{MinPhred: 20} }

// IsUsable reports whether a quality vector is good enough to cluster.
func (p QualityPolicy) IsUsable(q []int) bool {
	if len(q) == 0 {
		return false
	}
	for _, v := range q {
		if v < p.MinPhred {
			return false
		}
	}
	return true
}

// MeanQuality returns the mean phred score of a vector.
func (p QualityPolicy) MeanQuality(q []int) float64 {
	if len(q) == 0 {
		return 0
	}
	sum := 0
	for _, v := range q {
		sum += v
	}
	return float64(sum) / float64(len(q))
}
