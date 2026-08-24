// Package snapshot publishes an immutable discrimination result for a lineage.
// A published snapshot freezes the input reads, clusters, candidates and the
// ancestor assumption so the evidence can be audited later and cannot be
// silently edited.
package snapshot

import (
	"encoding/json"
	"fmt"
	"time"

	"task237-yeastbarcode/internal/model"
	"task237-yeastbarcode/internal/store"
)

// Service builds and stores discrimination snapshots.
type Service struct {
	st *store.Store
}

// New constructs a snapshot Service.
func New(st *store.Store) *Service { return &Service{st: st} }

// Result is the frozen payload carried by a snapshot.
type Result struct {
	LineageID   string                    `json:"lineage_id"`
	GeneratedAt string                    `json:"generated_at"`
	Clusters    []*model.CorrectionCluster `json:"clusters"`
	Candidates  []*model.ContaminationCandidate `json:"candidates"`
	Confirmed   int                       `json:"confirmed"`
	Rejected    int                       `json:"rejected"`
	Pending     int                       `json:"pending"`
}

// Publish builds a snapshot from the current state of a lineage and stores it
// as draft (caller may then confirm publication). If a prior published snapshot
// exists it is superseded.
func (s *Service) Publish(lineageID, summary string) (*model.DiscriminationSnapshot, error) {
	clusters, err := s.st.ListClusters(lineageID)
	if err != nil {
		return nil, err
	}
	cands, err := s.st.ListCandidates(lineageID)
	if err != nil {
		return nil, err
	}
	res := Result{
		LineageID:   lineageID,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Clusters:    clusters,
		Candidates:  cands,
	}
	for _, c := range cands {
		switch c.Status {
		case model.CandConfirmed:
			res.Confirmed++
		case model.CandRejected:
			res.Rejected++
		default:
			res.Pending++
		}
	}
	payload, err := json.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("snapshot: marshal: %w", err)
	}
	snap := &model.DiscriminationSnapshot{
		ID:         model.ClusterHash(lineageID, len(clusters), "snap")[:16],
		LineageID:  lineageID,
		Status:     model.SnapDraft,
		Summary:    summary,
		ResultJSON: string(payload),
		CreatedAt:  time.Now().UTC(),
	}
	// supersede any previously published snapshot
	prev, err := s.st.ListSnapshots(lineageID)
	if err != nil {
		return nil, err
	}
	if err := s.st.SaveSnapshot(snap); err != nil {
		return nil, err
	}
	for _, p := range prev {
		if p.Status == model.SnapPublished {
			if err := s.st.SupersedeSnapshot(p.ID, snap.ID); err != nil {
				return nil, err
			}
		}
	}
	return snap, nil
}

// Confirm marks a draft snapshot as published.
func (s *Service) Confirm(id string) error {
	snap, err := s.st.GetSnapshot(id)
	if err != nil {
		return err
	}
	if !model.ValidSnapshotTransition(snap.Status, model.SnapPublished) {
		return fmt.Errorf("snapshot: invalid transition %s->published", snap.Status)
	}
	snap.Status = model.SnapPublished
	return s.st.SaveSnapshot(snap)
}

// GetResult decodes a snapshot's frozen result payload.
func (s *Service) GetResult(id string) (*Result, error) {
	snap, err := s.st.GetSnapshot(id)
	if err != nil {
		return nil, err
	}
	var r Result
	if err := json.Unmarshal([]byte(snap.ResultJSON), &r); err != nil {
		return nil, fmt.Errorf("snapshot: unmarshal: %w", err)
	}
	return &r, nil
}
