package correction

import (
	"testing"

	"task237-yeastbarcode/internal/model"
	"task237-yeastbarcode/internal/read"
	"task237-yeastbarcode/internal/store"
)

func goodQuality(n int) []int {
	q := make([]int, n)
	for i := range q {
		q[i] = 35
	}
	return q
}

func ingest(t *testing.T, st *store.Store, lid string, gen int, bc string) {
	t.Helper()
	if _, err := read.New(st).IngestRead(lid, gen, bc, goodQuality(len(bc))); err != nil && err != model.ErrDuplicate {
		t.Fatalf("ingest %q: %v", bc, err)
	}
}

func countAncestor(t *testing.T, st *store.Store, lid string) int {
	t.Helper()
	clusters, err := st.ListClusters(lid)
	if err != nil {
		t.Fatalf("list clusters: %v", err)
	}
	n := 0
	for _, c := range clusters {
		if c.IsAncestor {
			n++
		}
	}
	return n
}

// lockAncestorGen marks every cluster of the given generation as an ancestor,
// mirroring what lineage.LockAncestor persists, without importing lineage.
func lockAncestorGen(t *testing.T, st *store.Store, lid string, gen int) {
	t.Helper()
	clusters, err := st.ListClusters(lid)
	if err != nil {
		t.Fatalf("list clusters: %v", err)
	}
	for _, c := range clusters {
		if c.Generation == gen {
			c.IsAncestor = true
			if err := st.SaveCluster(c); err != nil {
				t.Fatalf("save cluster: %v", err)
			}
		}
	}
}

// TestRerunCorrectionKeepsAncestorLock guards against the regression where
// re-running correction of the already-locked ancestor generation wipes the
// is_ancestor flag, leaving the lineage without its reference baseline.
func TestRerunCorrectionKeepsAncestorLock(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/correction-rerun.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	lid := "L-rerun"
	if err := st.SaveLineage(&model.CultureLineage{ID: lid, Name: "rerun", Status: model.LineageSequencing}); err != nil {
		t.Fatal(err)
	}

	ancestor := "ACGTACGTACGTACGT"
	for i := 0; i < 10; i++ {
		ingest(t, st, lid, 0, ancestor)
	}

	readSvc := read.New(st)
	corr := New(st, readSvc, DefaultConfig())

	// first correction + lock the ancestor generation
	if _, err := corr.ClusterGeneration(lid, 0); err != nil {
		t.Fatal(err)
	}
	lockAncestorGen(t, st, lid, 0)
	if n := countAncestor(t, st, lid); n != 1 {
		t.Fatalf("after lock: ancestor clusters=%d want 1", n)
	}

	// re-run correction of the SAME ancestor generation (the bug scenario)
	if _, err := corr.ClusterGeneration(lid, 0); err != nil {
		t.Fatal(err)
	}

	// the ancestor lock must survive the re-run
	if n := countAncestor(t, st, lid); n != 1 {
		t.Fatalf("after re-correction: ancestor clusters=%d want 1 (lock was wiped)", n)
	}
	gen, locked, err := st.AncestorGeneration(lid)
	if err != nil {
		t.Fatalf("ancestor generation: %v", err)
	}
	if !locked || gen != 0 {
		t.Fatalf("ancestor lock lost after re-run: locked=%v gen=%d", locked, gen)
	}
	barcodes, err := st.LockedAncestorBarcodes(lid)
	if err != nil {
		t.Fatalf("locked ancestor barcodes: %v", err)
	}
	if !barcodes[ancestor] {
		t.Fatalf("ancestor barcode %q not preserved as locked: %#v", ancestor, barcodes)
	}
}

// TestRerunCorrectionWithoutLockDoesNotMarkAncestor ensures re-correction of a
// generation that was never locked does not invent ancestor marks.
func TestRerunCorrectionWithoutLockDoesNotMarkAncestor(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/correction-nolock.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	lid := "L-nolock"
	if err := st.SaveLineage(&model.CultureLineage{ID: lid, Name: "nolock", Status: model.LineageSequencing}); err != nil {
		t.Fatal(err)
	}
	ingest(t, st, lid, 0, "ACGTACGTACGTACGT")
	corr := New(st, read.New(st), DefaultConfig())
	if _, err := corr.ClusterGeneration(lid, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := corr.ClusterGeneration(lid, 0); err != nil {
		t.Fatal(err)
	}
	if n := countAncestor(t, st, lid); n != 0 {
		t.Fatalf("unexpected ancestor clusters=%d want 0", n)
	}
}
