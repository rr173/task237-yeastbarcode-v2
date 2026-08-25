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

// A cross-generation shortcut edge (e.g. gen 0 -> gen 2 alongside an existing
// 0 -> 1 -> 2 chain) is a legal parallel path, not a cycle, and must be accepted.
func TestDetectCycleAcceptsShortcutEdge(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/shortcut.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for _, e := range []model.GenerationEdge{
		{LineageID: "L", ParentGeneration: 0, ChildGeneration: 1},
		{LineageID: "L", ParentGeneration: 1, ChildGeneration: 2},
		{LineageID: "L", ParentGeneration: 2, ChildGeneration: 3},
	} {
		if err := st.SaveGenerationEdge(e); err != nil {
			t.Fatal(err)
		}
	}
	svc := New(st)
	for _, e := range []model.GenerationEdge{
		{LineageID: "L", ParentGeneration: 0, ChildGeneration: 2},
		{LineageID: "L", ParentGeneration: 0, ChildGeneration: 3},
	} {
		if err := svc.DetectCycle(e.LineageID, e); err != nil {
			t.Fatalf("shortcut %d->%d wrongly rejected: %v", e.ParentGeneration, e.ChildGeneration, err)
		}
	}
}

// Re-submitting an edge that already exists must not be flagged as a cycle;
// idempotent duplication is the store layer's concern, not a graph violation.
func TestDetectCycleDoesNotFlagDuplicate(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/dup.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	e := model.GenerationEdge{LineageID: "L", ParentGeneration: 0, ChildGeneration: 1}
	if err := st.SaveGenerationEdge(e); err != nil {
		t.Fatal(err)
	}
	if err := New(st).DetectCycle(e.LineageID, e); err != nil {
		t.Fatalf("duplicate edge wrongly flagged as cycle: %v", err)
	}
}

// A genuine back-edge that closes a loop must still be rejected.
func TestDetectCycleRejectsBackEdge(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/backedge.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for _, e := range []model.GenerationEdge{
		{LineageID: "L", ParentGeneration: 0, ChildGeneration: 1},
		{LineageID: "L", ParentGeneration: 1, ChildGeneration: 2},
	} {
		if err := st.SaveGenerationEdge(e); err != nil {
			t.Fatal(err)
		}
	}
	// 2 -> 0 closes the loop 0 -> 1 -> 2 -> 0; must be rejected.
	err = New(st).DetectCycle("L", model.GenerationEdge{LineageID: "L", ParentGeneration: 2, ChildGeneration: 0})
	if err != model.ErrGenerationCycle {
		t.Fatalf("back-edge 2->0 must be rejected, got %v", err)
	}
}
