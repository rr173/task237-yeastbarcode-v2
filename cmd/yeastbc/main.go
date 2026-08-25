// Command yeastbc is the entry point for the yeast barcode pedigree
// contamination discriminator. It serves an HTTP API and supports a --smoke-test
// mode that exercises the full pipeline against a temporary SQLite database and
// verifies persistence + restart recovery, then exits 0.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"task237-yeastbarcode/internal/httpapi"
	"task237-yeastbarcode/internal/model"
	"task237-yeastbarcode/internal/service"
	"task237-yeastbarcode/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dbPath := flag.String("db", "yeastbc.db", "SQLite database path")
	smoke := flag.Bool("smoke-test", false, "run the end-to-end smoke test and exit")
	flag.Parse()

	if *smoke {
		if err := runSmokeTest(); err != nil {
			log.Fatalf("smoke-test failed: %v", err)
		}
		fmt.Println("smoke-test: PASS")
		os.Exit(0)
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	svc := service.New(st)
	srv := &http.Server{
		Addr:              *addr,
		Handler:           httpapi.New(svc).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("yeastbc listening on %s (db=%s)", *addr, *dbPath)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// runSmokeTest exercises the full pipeline with a temp DB, then re-opens the DB
// to prove persistence and restart recovery.
func runSmokeTest() error {
	dir, err := os.MkdirTemp("", "yeastbc-smoke")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	db := filepath.Join(dir, "smoke.db")

	// --- first session: build and analyze ---
	st, err := store.Open(db)
	if err != nil {
		return err
	}
	svc := service.New(st)
	if _, err := svc.CreateLineage("L1", "strain-A"); err != nil {
		return err
	}
	if _, err := svc.TransitionLineage("L1", "sequencing"); err != nil {
		return err
	}
	// founding generation 0 with a known ancestor barcode
	ancestor := "ACGTACGTACGTACGT"
	for i := 0; i < 20; i++ {
		if err := ingest(svc, "L1", 0, ancestor, goodQuality(16)); err != nil {
			return err
		}
	}
	// a single sequencing-error variant (within edit distance) should collapse
	if _, err := svc.IngestRead("L1", 0, "ACGTACGTACGTACGA", goodQuality(16)); err != nil {
		return err
	}
	// child generation 1 inherits ancestor, plus a foreign barcode (contamination)
	if err := svc.AddGenerationEdge(model.GenerationEdge{LineageID: "L1", ParentGeneration: 0, ChildGeneration: 1}); err != nil {
		return err
	}
	for i := 0; i < 18; i++ {
		if err := ingest(svc, "L1", 1, ancestor, goodQuality(16)); err != nil {
			return err
		}
	}
	foreign := "TTTTAAAACCCCGGGG"
	for i := 0; i < 5; i++ {
		if err := ingest(svc, "L1", 1, foreign, goodQuality(16)); err != nil {
			return err
		}
	}
	// a low-quality read should be excluded, not clustered
	if _, err := svc.IngestRead("L1", 1, foreign, badQuality(16)); err != nil {
		return err
	}
	if _, err := svc.CorrectGeneration("L1", 0); err != nil {
		return err
	}
	if err := svc.LockAncestor("L1", 0); err != nil {
		return err
	}
	if _, err := svc.CorrectGeneration("L1", 1); err != nil {
		return err
	}
	cands, err := svc.AnalyzeGeneration("L1", 1)
	if err != nil {
		return err
	}
	if len(cands) == 0 {
		return fmt.Errorf("expected at least one contamination candidate, got 0")
	}
	foundForeign := false
	for _, c := range cands {
		if c.Barcode == foreign && c.Status == "generated" {
			foundForeign = true
		}
	}
	if !foundForeign {
		return fmt.Errorf("foreign barcode not detected as candidate")
	}
	// confirm a candidate and publish a snapshot
	if err := svc.DecideCandidate(cands[0].ID, true, "confirmed by reviewer"); err != nil {
		return err
	}
	snap, err := svc.PublishSnapshot("L1", "smoke snapshot")
	if err != nil {
		return err
	}
	if err := svc.ConfirmSnapshot(snap.ID); err != nil {
		return err
	}
	// seal the lineage
	if _, err := svc.TransitionLineage("L1", "sealed"); err != nil {
		return err
	}
	if err := st.Close(); err != nil {
		return err
	}

	// --- second session: reopen DB, verify recovery ---
	st2, err := store.Open(db)
	if err != nil {
		return err
	}
	defer st2.Close()
	svc2 := service.New(st2)
	l, err := svc2.Store.GetLineage("L1")
	if err != nil {
		return err
	}
	if l.Status != "sealed" {
		return fmt.Errorf("lineage not recovered: status=%s", l.Status)
	}
	reads, err := svc2.Store.ListReadsByLineage("L1")
	if err != nil {
		return err
	}
	if len(reads) == 0 {
		return fmt.Errorf("reads not recovered after restart")
	}
	cands2, err := svc2.Store.ListCandidates("L1")
	if err != nil {
		return err
	}
	if len(cands2) == 0 {
		return fmt.Errorf("candidates not recovered after restart")
	}
	return nil
}

// ingest wraps IngestRead and treats idempotent duplicates as success.
func ingest(svc *service.Services, lid string, gen int, bc string, q []int) error {
	if _, err := svc.IngestRead(lid, gen, bc, q); err != nil && err != model.ErrDuplicate {
		return err
	}
	return nil
}

func goodQuality(n int) []int {
	q := make([]int, n)
	for i := range q {
		q[i] = 35
	}
	return q
}

func badQuality(n int) []int {
	q := make([]int, n)
	for i := range q {
		q[i] = 5 // below MinPhred
	}
	return q
}
