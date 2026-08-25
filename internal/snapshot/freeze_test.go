package snapshot

import (
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
