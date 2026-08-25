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
