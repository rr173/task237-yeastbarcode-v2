// Package discriminate is the analytical core: it compares the corrected
// barcode set of a generation against the inherited expectation (ancestor +
// parent barcodes) and flags foreign barcodes whose frequency exceeds a low
// contamination threshold as contamination candidates. It distinguishes true
// cross-contamination from sequencing noise (already filtered in correction)
// and from legitimate mutation (within inheritance path).
package discriminate

// Config controls contamination thresholds.
type Config struct {
	// MinFrequency is the minimum observed frequency of a foreign barcode for
	// it to be considered a candidate (low-frequency threshold). Barcodes seen
	// only once in a huge read set may still be noise; this guards the floor.
	MinFrequency float64
	// InheritTolerance allows a barcode to deviate by up to this many
	// Hamming substitutions from an inherited barcode and still count as the
	// same clone (captures natural mutation vs foreign contamination).
	InheritTolerance int
}

// DefaultConfig returns the standard discrimination config.
func DefaultConfig() Config {
	return Config{MinFrequency: 0.01, InheritTolerance: 1}
}
