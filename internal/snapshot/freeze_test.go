package snapshot

import (
	"sync"
	"testing"

	"task237-yeastbarcode/internal/model"
	"task237-yeastbarcode/internal/store"
)

func TestPublishConfirmAndReadFrozenResult(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/snapshot.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SaveLineage(&model.CultureLineage{ID: "L-snapshot", Name: "snapshot", Status: model.LineageFiled}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveCluster(&model.CorrectionCluster{ID: "cluster", LineageID: "L-snapshot", Generation: 0, CanonicalBarcode: "ACGT", ReadCount: 3}); err != nil {
		t.Fatal(err)
	}
	svc := New(st)
	snap, err := svc.Publish("L-snapshot", "review result")
	if err != nil {
		t.Fatal(err)
	}
	if snap.Status != model.SnapDraft {
		t.Fatalf("new snapshot status=%s", snap.Status)
	}
	if err := svc.Confirm(snap.ID); err != nil {
		t.Fatal(err)
	}
	result, err := svc.GetResult(snap.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.LineageID != "L-snapshot" || len(result.Clusters) != 1 {
		t.Fatalf("unexpected frozen result: %+v", result)
	}
}

func TestConcurrentSnapshotConfirmationPublishesOnce(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/snapshot-race.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SaveLineage(&model.CultureLineage{ID: "L-snapshot-race", Name: "snapshot race", Status: model.LineageFiled}); err != nil {
		t.Fatal(err)
	}
	snap, err := New(st).Publish("L-snapshot-race", "race")
	if err != nil {
		t.Fatal(err)
	}
	svc := New(st)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- svc.Confirm(snap.ID)
		}()
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
		t.Fatalf("confirmation outcomes successes=%d conflicts=%d", successes, conflicts)
	}
}

// TestRepeatedPublishYieldsIndependentSnapshots guards the contract that two
// discrimination publishes for the same lineage each produce its own snapshot
// identity and retain its own summary — a later publish must not reuse the
// earlier identity nor overwrite the earlier summary/result.
func TestRepeatedPublishYieldsIndependentSnapshots(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/snapshot-republish.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SaveLineage(&model.CultureLineage{ID: "L-republish", Name: "republish", Status: model.LineageFiled}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveCluster(&model.CorrectionCluster{ID: "cluster", LineageID: "L-republish", Generation: 0, CanonicalBarcode: "ACGT", ReadCount: 3}); err != nil {
		t.Fatal(err)
	}
	svc := New(st)

	first, err := svc.Publish("L-republish", "first summary")
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Publish("L-republish", "second summary")
	if err != nil {
		t.Fatal(err)
	}

	if first.ID == "" || second.ID == "" {
		t.Fatalf("snapshot ids must be non-empty: first=%q second=%q", first.ID, second.ID)
	}
	if first.ID == second.ID {
		t.Fatalf("repeated publish reused the same snapshot id %q; each publish must get its own identity", first.ID)
	}
	if first.Summary != "first summary" {
		t.Fatalf("first snapshot summary mutated by later publish: got %q", first.Summary)
	}
	if second.Summary != "second summary" {
		t.Fatalf("second snapshot summary not retained: got %q", second.Summary)
	}

	rows, err := st.ListSnapshots("L-republish")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 independent snapshots persisted, got %d", len(rows))
	}
	summaries := map[string]bool{}
	ids := map[string]bool{}
	for _, r := range rows {
		ids[r.ID] = true
		summaries[r.Summary] = true
	}
	if !ids[first.ID] || !ids[second.ID] {
		t.Fatalf("both published ids must be persisted: got %v", ids)
	}
	if !summaries["first summary"] || !summaries["second summary"] {
		t.Fatalf("both summaries must be retained independently: got %v", summaries)
	}

	// The first snapshot's frozen result must survive the second publish
	// untouched: a later publish must not overwrite an earlier snapshot's
	// content even when its status changes (it becomes superseded here).
	res, err := svc.GetResult(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.LineageID != "L-republish" || len(res.Clusters) != 1 {
		t.Fatalf("first snapshot frozen result overwritten/lost: %+v", res)
	}
}

// TestSaveSnapshotRejectsIdentityCollision asserts the persistence layer
// refuses to silently overwrite an existing snapshot when an id collides, so a
// later publish can never rewrite an older published summary/result.
func TestSaveSnapshotRejectsIdentityCollision(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/snapshot-collision.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SaveLineage(&model.CultureLineage{ID: "L-collide", Name: "collide", Status: model.LineageFiled}); err != nil {
		t.Fatal(err)
	}
	original := &model.DiscriminationSnapshot{
		ID:         "snap-collide",
		LineageID:  "L-collide",
		Status:     model.SnapPublished,
		Summary:    "original summary",
		ResultJSON: `{"lineage_id":"L-collide"}`,
	}
	if err := st.SaveSnapshot(original); err != nil {
		t.Fatalf("save original: %v", err)
	}
	// a second snapshot attempting to reuse the same id must fail loudly, not
	// overwrite the original summary/result.
	clone := &model.DiscriminationSnapshot{
		ID:         "snap-collide",
		LineageID:  "L-collide",
		Status:     model.SnapPublished,
		Summary:    "overwriting summary",
		ResultJSON: `{"lineage_id":"L-collide","overwritten":true}`,
	}
	if err := st.SaveSnapshot(clone); err != model.ErrDuplicate {
		t.Fatalf("expected model.ErrDuplicate on id collision, got %v", err)
	}
	got, err := st.GetSnapshot("snap-collide")
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary != "original summary" {
		t.Fatalf("original snapshot summary was overwritten: got %q", got.Summary)
	}
	if got.ResultJSON != `{"lineage_id":"L-collide"}` {
		t.Fatalf("original snapshot result was overwritten: got %q", got.ResultJSON)
	}
}
