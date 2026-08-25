// Package httpapi exposes the yeast barcode discriminator over HTTP. All routes
// are prefixed with /api. It maps domain errors to JSON status codes and never
// leaks the raw SQLite layer to callers.
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"task237-yeastbarcode/internal/model"
	"task237-yeastbarcode/internal/service"
)

// Server holds the HTTP dependencies.
type Server struct {
	svc *service.Services
}

// New constructs the HTTP server.
func New(svc *service.Services) *Server { return &Server{svc: svc} }

// Handler returns the configured mux.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/healthz", s.handleHealthz)
	mux.HandleFunc("/api/self-check", s.handleSelfCheck)

	// lineage
	mux.HandleFunc("/api/lineages", s.handleLineages)     // GET list, POST create
	mux.HandleFunc("/api/lineages/", s.handleLineageByID) // GET /{id}, POST /{id}/transition

	// reads & generations
	mux.HandleFunc("/api/lineages/{id}/reads", s.handleReads)                                 // POST ingest, GET list
	mux.HandleFunc("/api/lineages/{id}/generations/{gen}/reads", s.handleGenerationReads)     // GET list gen
	mux.HandleFunc("/api/lineages/{id}/edges", s.handleEdges)                                 // POST add edge
	mux.HandleFunc("/api/lineages/{id}/edges/list", s.handleEdgeList)                         // GET generation edges
	mux.HandleFunc("/api/lineages/{id}/generations/{gen}/correct", s.handleCorrect)           // POST correct
	mux.HandleFunc("/api/lineages/{id}/generations/{gen}/clusters", s.handleClusters)         // GET clusters
	mux.HandleFunc("/api/lineages/{id}/generations/{gen}/quality", s.handleGenerationQuality) // GET quality summary
	mux.HandleFunc("/api/lineages/{id}/generations/{gen}/expected", s.handleExpectedBarcodes) // GET inherited set
	mux.HandleFunc("/api/lineages/{id}/ancestor", s.handleLockAncestor)                       // POST lock ancestor
	mux.HandleFunc("/api/lineages/{id}/ancestor/barcodes", s.handleAncestorBarcodes)          // GET locked set

	// discrimination
	mux.HandleFunc("/api/lineages/{id}/generations/{gen}/analyze", s.handleAnalyze) // POST analyze
	mux.HandleFunc("/api/lineages/{id}/candidates", s.handleCandidates)             // GET list
	mux.HandleFunc("/api/candidates/{cid}/decide", s.handleDecideCandidate)         // POST decide
	mux.HandleFunc("/api/candidates/{cid}", s.handleCandidateByID)                  // GET detail

	// snapshots
	mux.HandleFunc("/api/lineages/{id}/snapshots", s.handleSnapshots)             // POST publish, GET list
	mux.HandleFunc("/api/snapshots/{sid}", s.handleSnapshotByID)                  // GET, POST confirm
	mux.HandleFunc("/api/lineages/{id}/snapshots/{sid}", s.handleLineageSnapshot) // GET lineage-scoped detail
	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleSelfCheck(w http.ResponseWriter, r *http.Request) {
	counts, err := s.svc.SelfCheck()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, counts)
}

func (s *Server) handleLineages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.svc.Store.ListLineages()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	case http.MethodPost:
		var body struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		l, err := s.svc.CreateLineage(body.ID, body.Name)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, l)
	default:
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
	}
}

func (s *Server) handleLineageByID(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/api/lineages/")
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("missing lineage id"))
		return
	}
	switch r.Method {
	case http.MethodGet:
		l, err := s.svc.Store.GetLineage(id)
		if err != nil {
			writeError(w, statusNotFound(err), err)
			return
		}
		writeJSON(w, http.StatusOK, l)
	case http.MethodPost:
		// expecting /api/lineages/{id}/transition
		if !strings.HasSuffix(r.URL.Path, "/transition") {
			writeError(w, http.StatusNotFound, fmt.Errorf("not found"))
			return
		}
		var body struct {
			To string `json:"to"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		l, err := s.svc.TransitionLineage(id, model.LineageStatus(body.To))
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, l)
	default:
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
	}
}

func (s *Server) handleReads(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/api/lineages/")
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("missing lineage id"))
		return
	}
	switch r.Method {
	case http.MethodGet:
		list, err := s.svc.Store.ListReadsByLineage(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	case http.MethodPost:
		var body struct {
			Generation int    `json:"generation"`
			Barcode    string `json:"barcode"`
			Quality    []int  `json:"quality"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		rd, err := s.svc.IngestRead(id, body.Generation, body.Barcode, body.Quality)
		if err != nil {
			if err == model.ErrDuplicate {
				writeJSON(w, http.StatusOK, map[string]any{"duplicate": true, "read": rd})
				return
			}
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, rd)
	default:
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
	}
}

func (s *Server) handleGenerationReads(w http.ResponseWriter, r *http.Request) {
	id, gen, ok := extractGen(r.URL.Path)
	if !ok {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid path"))
		return
	}
	list, err := s.svc.Store.ListReadsByGeneration(id, gen)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleEdges(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	id := extractID(r.URL.Path, "/api/lineages/")
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("missing lineage id"))
		return
	}
	var body struct {
		Parent int `json:"parent_generation"`
		Child  int `json:"child_generation"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	e := model.GenerationEdge{LineageID: id, ParentGeneration: body.Parent, ChildGeneration: body.Child}
	if err := s.svc.AddGenerationEdge(e); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, e)
}

func (s *Server) handleEdgeList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	id := extractID(r.URL.Path, "/api/lineages/")
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("missing lineage id"))
		return
	}
	edges, err := s.svc.Store.ListGenerationEdges(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, edges)
}

func (s *Server) handleCorrect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	id, gen, ok := extractGen(r.URL.Path)
	if !ok {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid path"))
		return
	}
	clusters, err := s.svc.CorrectGeneration(id, gen)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, clusters)
}

func (s *Server) handleClusters(w http.ResponseWriter, r *http.Request) {
	id, gen, ok := extractGen(r.URL.Path)
	if !ok {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid path"))
		return
	}
	all, err := s.svc.Store.ListClusters(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := []*model.CorrectionCluster{}
	for _, c := range all {
		if c.Generation == gen {
			out = append(out, c)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGenerationQuality(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	id, gen, ok := extractGen(r.URL.Path)
	if !ok {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid path"))
		return
	}
	result, err := s.svc.GenerationQuality(id, gen)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleExpectedBarcodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	id, gen, ok := extractGen(r.URL.Path)
	if !ok {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid path"))
		return
	}
	result, err := s.svc.ExpectedBarcodes(id, gen)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleLockAncestor(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	id := extractID(r.URL.Path, "/api/lineages/")
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("missing lineage id"))
		return
	}
	var body struct {
		Generation int `json:"generation"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.svc.LockAncestor(id, body.Generation); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "locked"})
}

func (s *Server) handleAncestorBarcodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	id := extractID(r.URL.Path, "/api/lineages/")
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("missing lineage id"))
		return
	}
	barcodes, err := s.svc.AncestorBarcodes(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, barcodes)
}

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	id, gen, ok := extractGen(r.URL.Path)
	if !ok {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid path"))
		return
	}
	cands, err := s.svc.AnalyzeGeneration(id, gen)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, cands)
}

func (s *Server) handleCandidates(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/api/lineages/")
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("missing lineage id"))
		return
	}
	list, err := s.svc.Store.ListCandidates(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleDecideCandidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	cid := extractID(r.URL.Path, "/api/candidates/")
	cid = strings.TrimSuffix(cid, "/decide")
	if cid == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("missing candidate id"))
		return
	}
	var body struct {
		Confirm bool   `json:"confirm"`
		Note    string `json:"note"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.svc.DecideCandidate(cid, body.Confirm, body.Note); err != nil {
		writeError(w, statusForCandidateDecision(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "decided"})
}

func (s *Server) handleCandidateByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	id := extractID(r.URL.Path, "/api/candidates/")
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("missing candidate id"))
		return
	}
	candidate, err := s.svc.GetCandidate(id)
	if err != nil {
		writeError(w, statusNotFound(err), err)
		return
	}
	writeJSON(w, http.StatusOK, candidate)
}

func (s *Server) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/api/lineages/")
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("missing lineage id"))
		return
	}
	switch r.Method {
	case http.MethodGet:
		list, err := s.svc.Store.ListSnapshots(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
	case http.MethodPost:
		var body struct {
			Summary string `json:"summary"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		snap, err := s.svc.PublishSnapshot(id, body.Summary)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusCreated, snap)
	default:
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
	}
}

func (s *Server) handleSnapshotByID(w http.ResponseWriter, r *http.Request) {
	sid := extractID(r.URL.Path, "/api/snapshots/")
	if sid == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("missing snapshot id"))
		return
	}
	switch r.Method {
	case http.MethodGet:
		res, err := s.svc.Snapshot.GetResult(sid)
		if err != nil {
			writeError(w, statusNotFound(err), err)
			return
		}
		writeJSON(w, http.StatusOK, res)
	case http.MethodPost:
		if !strings.HasSuffix(r.URL.Path, "/confirm") {
			writeError(w, http.StatusNotFound, fmt.Errorf("not found"))
			return
		}
		if err := s.svc.ConfirmSnapshot(sid); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "published"})
	default:
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
	}
}

func (s *Server) handleLineageSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 5 || parts[0] != "api" || parts[1] != "lineages" || parts[3] != "snapshots" || parts[2] == "" || parts[4] == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid path"))
		return
	}
	snap, err := s.svc.Store.GetSnapshot(parts[4])
	if err != nil {
		writeError(w, statusNotFound(err), err)
		return
	}
	if snap.LineageID != parts[2] {
		writeError(w, http.StatusNotFound, model.ErrNotFound)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// ---- helpers ----

func decodeJSON(r *http.Request, v any) error {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func statusNotFound(err error) int {
	if err == model.ErrNotFound {
		return http.StatusNotFound
	}
	return http.StatusBadRequest
}

// statusForCandidateDecision maps decide errors to status codes. A concurrent
// verdict that lost the compare-and-set race is a conflict, not a bad request.
func statusForCandidateDecision(err error) int {
	switch {
	case errors.Is(err, model.ErrStateConflict):
		return http.StatusConflict
	case err == model.ErrNotFound:
		return http.StatusNotFound
	default:
		return http.StatusBadRequest
	}
}

func extractID(path, prefix string) string {
	rest := strings.TrimPrefix(path, prefix)
	rest = strings.Trim(rest, "/")
	if i := strings.Index(rest, "/"); i >= 0 {
		return rest[:i]
	}
	return rest
}

func extractGen(path string) (string, int, bool) {
	// path like /api/lineages/{id}/generations/{gen}/...
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// expect ["api","lineages",id,"generations",gen,...]
	if len(parts) < 5 || parts[0] != "api" || parts[1] != "lineages" || parts[3] != "generations" {
		return "", 0, false
	}
	id := parts[2]
	gen, err := strconv.Atoi(parts[4])
	if err != nil {
		return "", 0, false
	}
	return id, gen, true
}
