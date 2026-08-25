package lineage

import (
	"errors"
	"testing"
	"time"

	"task237-yeastbarcode/internal/model"
	"task237-yeastbarcode/internal/store"
)

// TestLockAncestorRejectsSecondFoundingGeneration enforces that a lineage may
// lock at most one founding generation as the ancestor. Locking generation 0
// succeeds; a subsequent attempt to lock generation 1 must be rejected with
// model.ErrAncestorAlreadyLocked, and AncestorBarcodes must still return only
// the generation-0 barcode.
func TestLockAncestorRejectsSecondFoundingGeneration(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/ancestor.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SaveLineage(&model.CultureLineage{ID: "L-anc", Name: "ancestor", Status: model.LineageFiled, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	// generation 0 (founding) and generation 1 (child), each one cluster
	if err := st.SaveCluster(&model.CorrectionCluster{ID: "g0", LineageID: "L-anc", Generation: 0, CanonicalBarcode: "AAAA", ReadCount: 10}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveCluster(&model.CorrectionCluster{ID: "g1", LineageID: "L-anc", Generation: 1, CanonicalBarcode: "TTTT", ReadCount: 8}); err != nil {
		t.Fatal(err)
	}
	svc := New(st)

	if err := svc.LockAncestor("L-anc", 0); err != nil {
		t.Fatalf("first lock generation 0 failed: %v", err)
	}
	if err := svc.LockAncestor("L-anc", 1); !errors.Is(err, model.ErrAncestorAlreadyLocked) {
		t.Fatalf("second lock generation 1 error=%v want ErrAncestorAlreadyLocked", err)
	}
	barcodes, err := svc.AncestorBarcodes("L-anc")
	if err != nil {
		t.Fatalf("ancestor barcodes: %v", err)
	}
	if !barcodes["AAAA"] {
		t.Fatalf("generation-0 barcode missing from ancestor set: %#v", barcodes)
	}
	if barcodes["TTTT"] {
		t.Fatalf("generation-1 barcode leaked into ancestor set: %#v", barcodes)
	}
}

// TestLockAncestorIdempotentSameGeneration confirms that re-locking the already
// locked generation is idempotent (no rejection, no change to the set).
func TestLockAncestorIdempotentSameGeneration(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/ancestor-idem.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SaveLineage(&model.CultureLineage{ID: "L-idem", Name: "idem", Status: model.LineageFiled, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveCluster(&model.CorrectionCluster{ID: "g0", LineageID: "L-idem", Generation: 0, CanonicalBarcode: "CCCC", ReadCount: 10}); err != nil {
		t.Fatal(err)
	}
	svc := New(st)

	if err := svc.LockAncestor("L-idem", 0); err != nil {
		t.Fatalf("first lock failed: %v", err)
	}
	if err := svc.LockAncestor("L-idem", 0); err != nil {
		t.Fatalf("idempotent re-lock failed: %v", err)
	}
	barcodes, err := svc.AncestorBarcodes("L-idem")
	if err != nil {
		t.Fatalf("ancestor barcodes: %v", err)
	}
	if len(barcodes) != 1 || !barcodes["CCCC"] {
		t.Fatalf("unexpected ancestor set after re-lock: %#v", barcodes)
	}
}
