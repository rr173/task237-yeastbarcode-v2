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
