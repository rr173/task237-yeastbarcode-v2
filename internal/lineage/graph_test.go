package lineage

import (
	"testing"

	"task237-yeastbarcode/internal/model"
	"task237-yeastbarcode/internal/store"
)

func TestExpectedBarcodesUsesParentClusters(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/lineage.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SaveGenerationEdge(model.GenerationEdge{LineageID: "L-lineage", ParentGeneration: 0, ChildGeneration: 1}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveCluster(&model.CorrectionCluster{ID: "parent", LineageID: "L-lineage", Generation: 0, CanonicalBarcode: "ACGT", ReadCount: 4}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveCluster(&model.CorrectionCluster{ID: "other", LineageID: "L-lineage", Generation: 2, CanonicalBarcode: "TTTT", ReadCount: 4}); err != nil {
		t.Fatal(err)
	}
	got, err := New(st).ExpectedBarcodes("L-lineage", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !got["ACGT"] || got["TTTT"] {
		t.Fatalf("unexpected inherited set: %#v", got)
	}
}

func TestValidateEdgeRejectsNonIncreasingGeneration(t *testing.T) {
	err := New(nil).ValidateEdge(model.GenerationEdge{LineageID: "L", ParentGeneration: 2, ChildGeneration: 2})
	if err != model.ErrGenerationCycle {
		t.Fatalf("error=%v want %v", err, model.ErrGenerationCycle)
	}
}
