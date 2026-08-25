package store

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"task237-yeastbarcode/internal/model"
)

func TestOpenReopenAndRollback(t *testing.T) {
	dbPath := t.TempDir() + "/yeast.db"
	st, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	lineage := &model.CultureLineage{ID: "L-store", Name: "store test", Status: model.LineageFiled, CreatedAt: time.Now().UTC()}
	if err := st.SaveLineage(lineage); err != nil {
		t.Fatal(err)
	}
	if err := st.InTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO lineages (id, name, status, created_at, sealed_at) VALUES (?, ?, ?, ?, ?)`, "L-rollback", "rollback", string(model.LineageFiled), time.Now().UTC().Format(time.RFC3339), ""); err != nil {
			return err
		}
		return errors.New("rollback sentinel")
	}); err == nil {
		t.Fatal("expected transaction callback failure")
	}
	if _, err := st.GetLineage("L-rollback"); err != model.ErrNotFound {
		t.Fatalf("rolled-back lineage lookup error=%v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	st, err = Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	got, err := st.GetLineage("L-store")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != lineage.Name || got.Status != model.LineageFiled {
		t.Fatalf("reopened lineage mismatch: %+v", got)
	}
}

// SaveGenerationEdge must be idempotent: re-submitting the same edge is a no-op,
// it must never error, and it must not create a duplicate row.
func TestSaveGenerationEdgeIdempotent(t *testing.T) {
	st, err := Open(t.TempDir() + "/edge.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SaveLineage(&model.CultureLineage{ID: "L-edge", Name: "edge", Status: model.LineageFiled, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	e := model.GenerationEdge{LineageID: "L-edge", ParentGeneration: 0, ChildGeneration: 1}
	if err := st.SaveGenerationEdge(e); err != nil {
		t.Fatalf("first save: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := st.SaveGenerationEdge(e); err != nil {
			t.Fatalf("duplicate save #%d: %v", i, err)
		}
	}
	edges, err := st.ListGenerationEdges("L-edge")
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected single edge after duplicates, got %d: %+v", len(edges), edges)
	}
}
