package model

import (
	"strings"
	"unicode"
)

// ValidateBarcode checks that a barcode only uses the allowed alphabet.
func ValidateBarcode(bc string) error {
	if bc == "" {
		return ErrBarcodeIllegal
	}
	for _, r := range bc {
		if !strings.ContainsRune(BarcodeAlphabet, r) {
			return ErrBarcodeIllegal
		}
		if unicode.IsLower(r) {
			// alphabet is upper-case only; lower-case is illegal
			return ErrBarcodeIllegal
		}
	}
	return nil
}

// ValidateQuality checks that quality scores match the barcode length and are
// within the 0..40 phred range.
func ValidateQuality(bc string, q []int) error {
	if len(q) != len([]rune(bc)) {
		return ErrQualityMissing
	}
	for _, v := range q {
		if v < 0 || v > 40 {
			return ErrQualityMissing
		}
	}
	return nil
}

// ValidateGenerationEdge checks basic sanity of a parent/child generation edge
// and returns ErrGenerationCycle when the edge would be non-increasing.
func ValidateGenerationEdge(e GenerationEdge) error {
	if e.ParentGeneration >= e.ChildGeneration {
		return ErrGenerationCycle
	}
	if e.ParentGeneration < 0 || e.ChildGeneration < 0 {
		return ErrGenerationCycle
	}
	return nil
}
