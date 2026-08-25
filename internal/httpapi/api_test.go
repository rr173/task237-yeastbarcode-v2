package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task237-yeastbarcode/internal/service"
	"task237-yeastbarcode/internal/store"
)

func TestHandlerHealthAndLineageLifecycle(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/api.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	server := httptest.NewServer(New(service.New(st)).Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/healthz")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status=%d", resp.StatusCode)
	}
	resp.Body.Close()

	body, _ := json.Marshal(map[string]string{"id": "L-api", "name": "API test"})
	resp, err = http.Post(server.URL+"/api/lineages", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status=%d", resp.StatusCode)
	}
	resp.Body.Close()

	resp, err = http.Get(server.URL + "/api/lineages/L-api")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get status=%d", resp.StatusCode)
	}
	resp.Body.Close()
}
