package discriminate

import (
	"sync"
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
	if err := st.SaveLineage(&model.CultureLineage{ID: "L-analysis", Name: "analysis", Status: model.LineageFiled}); err != nil {
		t.Fatal(err)
	}
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

// TestReanalyzePreservesVerdict is the regression guard for the bug where
// re-analysing an already-adjudicated generation reset decided candidates back
// to "generated" and dropped their verdict note. Because candidate ids are a
// stable hash of (lineage, generation, barcode), re-analysis upserts onto the
// existing rows; the upsert must refresh evidence metrics but never touch the
// human verdict (status / verdict_note / decided_at).
func TestReanalyzePreservesVerdict(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/reanalyze.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SaveLineage(&model.CultureLineage{ID: "L-reanalyze", Name: "reanalyze", Status: model.LineageFiled}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveGenerationEdge(model.GenerationEdge{LineageID: "L-reanalyze", ParentGeneration: 0, ChildGeneration: 1}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveCluster(&model.CorrectionCluster{ID: "ancestor-ra", LineageID: "L-reanalyze", Generation: 0, CanonicalBarcode: "ACGT", ReadCount: 10, IsAncestor: true}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveCluster(&model.CorrectionCluster{ID: "inherited-ra", LineageID: "L-reanalyze", Generation: 1, CanonicalBarcode: "ACGT", ReadCount: 8}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveCluster(&model.CorrectionCluster{ID: "foreign-ra", LineageID: "L-reanalyze", Generation: 1, CanonicalBarcode: "TTTT", ReadCount: 4}); err != nil {
		t.Fatal(err)
	}
	svc := New(st, DefaultConfig())

	// first analysis produces the candidate
	cands, err := svc.AnalyzeGeneration("L-reanalyze", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 1 || cands[0].Barcode != "TTTT" || cands[0].Status != model.CandGenerated {
		t.Fatalf("initial analysis unexpected candidates: %+v", cands)
	}
	candID := cands[0].ID

	// researcher rejects it with a note
	if err := svc.DecideCandidate(candID, false, "reviewed: residual error"); err != nil {
		t.Fatal(err)
	}
	decided, err := st.GetCandidate(candID)
	if err != nil {
		t.Fatal(err)
	}
	if decided.Status != model.CandRejected || decided.VerdictNote != "reviewed: residual error" {
		t.Fatalf("decision not persisted: %+v", decided)
	}

	// re-analyse the same generation (researcher returning to re-check work)
	reCands, err := svc.AnalyzeGeneration("L-reanalyze", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(reCands) != 1 || reCands[0].ID != candID {
		t.Fatalf("re-analysis should upsert onto the same candidate id, got: %+v", reCands)
	}
	if reCands[0].Status != model.CandRejected {
		t.Fatalf("re-analysis reset a rejected candidate to %s", reCands[0].Status)
	}
	if reCands[0].VerdictNote != "reviewed: residual error" {
		t.Fatalf("re-analysis dropped the verdict note: %q", reCands[0].VerdictNote)
	}
	// confirm the persistence layer agrees (not just the in-memory return slice)
	persisted, err := st.GetCandidate(candID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Status != model.CandRejected || persisted.VerdictNote != "reviewed: residual error" || persisted.DecidedAt == nil {
		t.Fatalf("re-analysis clobbered persisted verdict: %+v", persisted)
	}
}

func TestConcurrentCandidateDecisionsUseCompareAndSet(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/decision-race.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SaveLineage(&model.CultureLineage{ID: "L-decision-race", Name: "decision race", Status: model.LineageFiled}); err != nil {
		t.Fatal(err)
	}
	candidate := &model.ContaminationCandidate{ID: "candidate-race", LineageID: "L-decision-race", Generation: 1, Barcode: "TTTT", EvidenceScore: 0.8, Frequency: 0.2, Source: "test", Status: model.CandGenerated}
	if err := st.SaveCandidate(candidate); err != nil {
		t.Fatal(err)
	}
	svc := New(st, DefaultConfig())
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, confirm := range []bool{true, false} {
		wg.Add(1)
		go func(confirm bool) {
			defer wg.Done()
			errs <- svc.DecideCandidate(candidate.ID, confirm, "concurrent")
		}(confirm)
	}
	wg.Wait()
	close(errs)
	successes, conflicts := 0, 0
	for callErr := range errs {
		if callErr == nil {
			successes++
		} else {
			conflicts++
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("decision outcomes successes=%d conflicts=%d", successes, conflicts)
	}
}
