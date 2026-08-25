package discriminate

import (
	"testing"

	"task237-yeastbarcode/internal/model"
	"task237-yeastbarcode/internal/store"
)

func TestAnalyzeGenerationFindsForeignCluster(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/analysis.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SaveGenerationEdge(model.GenerationEdge{LineageID: "L-analysis", ParentGeneration: 0, ChildGeneration: 1}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveCluster(&model.CorrectionCluster{ID: "ancestor", LineageID: "L-analysis", Generation: 0, CanonicalBarcode: "ACGT", ReadCount: 10, IsAncestor: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveCluster(&model.CorrectionCluster{ID: "inherited", LineageID: "L-analysis", Generation: 1, CanonicalBarcode: "ACGT", ReadCount: 8}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveCluster(&model.CorrectionCluster{ID: "foreign", LineageID: "L-analysis", Generation: 1, CanonicalBarcode: "TTTT", ReadCount: 4}); err != nil {
		t.Fatal(err)
	}
	cands, err := New(st, DefaultConfig()).AnalyzeGeneration("L-analysis", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 1 || cands[0].Barcode != "TTTT" || cands[0].Status != model.CandGenerated {
		t.Fatalf("unexpected candidates: %+v", cands)
	}
}
