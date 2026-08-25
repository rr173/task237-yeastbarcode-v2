package model

import "testing"

func TestValidLineageTransition(t *testing.T) {
	if !ValidLineageTransition(LineageFiled, LineageSequencing) {
		t.Error("filed->sequencing should be valid")
	}
	if ValidLineageTransition(LineageSealed, LineageConfirmed) {
		t.Error("sealed is terminal")
	}
}

func TestValidateBarcode(t *testing.T) {
	if err := ValidateBarcode("ACGTN-ACGT"); err != nil {
		t.Errorf("valid barcode rejected: %v", err)
	}
	if err := ValidateBarcode("ACGTQ"); err != ErrBarcodeIllegal {
		t.Errorf("illegal char should be ErrBarcodeIllegal, got %v", err)
	}
}

func TestValidateQuality(t *testing.T) {
	if err := ValidateQuality("ACGT", []int{30, 30, 30, 30}); err != nil {
		t.Errorf("valid quality rejected: %v", err)
	}
	if err := ValidateQuality("ACGT", []int{30, 30, 30}); err != ErrQualityMissing {
		t.Errorf("length mismatch should be ErrQualityMissing")
	}
}
