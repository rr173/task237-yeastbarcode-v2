package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"task237-yeastbarcode/internal/service"
	"task237-yeastbarcode/internal/store"
)

// TestConcurrentDuplicateIngestIsDeduplicated verifies the concurrent-dedup
// contract: when an experimenter submits N byte-identical reads of the same
// generation at once, exactly one request succeeds (201 Created) and every
// other request is recognized as a duplicate read (200, {"duplicate":true}),
// and exactly one read row is persisted.
func TestConcurrentDuplicateIngestIsDeduplicated(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/concdup.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	svc := service.New(st)
	server := httptest.NewServer(New(svc).Handler())
	defer server.Close()

	// create a lineage to ingest into
	body, _ := json.Marshal(map[string]string{"id": "Lcdup", "name": "concurrent dup"})
	resp, err := http.Post(server.URL+"/api/lineages", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	quality := []int{35, 35, 35, 35}
	readBody, _ := json.Marshal(map[string]any{"generation": 0, "barcode": "ACGT", "quality": quality})

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	var created, duplicate int64
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			res, err := http.Post(server.URL+"/api/lineages/Lcdup/reads", "application/json", bytes.NewReader(readBody))
			if err != nil {
				return
			}
			var payload map[string]any
			_ = json.NewDecoder(res.Body).Decode(&payload)
			res.Body.Close()
			switch res.StatusCode {
			case http.StatusCreated:
				atomic.AddInt64(&created, 1)
			case http.StatusOK:
				if _, ok := payload["duplicate"]; ok {
					atomic.AddInt64(&duplicate, 1)
				}
			}
		}()
	}
	wg.Wait()

	reads, err := svc.Store.ListReadsByLineage("Lcdup")
	if err != nil {
		t.Fatal(err)
	}
	if len(reads) != 1 {
		t.Fatalf("want exactly 1 persisted read, got %d", len(reads))
	}
	if created != 1 {
		t.Fatalf("want exactly 1 Created response, got %d", created)
	}
	if duplicate != n-1 {
		t.Fatalf("want %d duplicate responses, got %d", n-1, duplicate)
	}
}
