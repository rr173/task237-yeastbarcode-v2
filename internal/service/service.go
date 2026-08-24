// Package service orchestrates the read/correction/lineage/discriminate/
// snapshot packages into the end-to-end business flow and enforces the
// lineage state machine. It is the single entry point used by the HTTP layer.
package service

import (
	"fmt"
	"time"

	"task237-yeastbarcode/internal/correction"
	"task237-yeastbarcode/internal/discriminate"
	"task237-yeastbarcode/internal/lineage"
	"task237-yeastbarcode/internal/model"
	"task237-yeastbarcode/internal/read"
	"task237-yeastbarcode/internal/snapshot"
	"task237-yeastbarcode/internal/store"
)

// Services bundles the domain services.
type Services struct {
	Store       *store.Store
	Read        *read.Service
	Correction  *correction.Service
	Lineage     *lineage.Service
	Discriminate *discriminate.Service
	Snapshot    *snapshot.Service
}

// New wires up all domain services over a single store.
func New(st *store.Store) *Services {
	readSvc := read.New(st)
	cfgCorr := correction.DefaultConfig()
	cfgDisc := discriminate.DefaultConfig()
	return &Services{
		Store:       st,
		Read:        readSvc,
		Correction:  correction.New(st, readSvc, cfgCorr),
		Lineage:     lineage.New(st),
		Discriminate: discriminate.New(st, cfgDisc),
		Snapshot:    snapshot.New(st),
	}
}

// CreateLineage creates a new culture lineage in the filed state.
func (s *Services) CreateLineage(id, name string) (*model.CultureLineage, error) {
	if id == "" {
		return nil, fmt.Errorf("service: empty lineage id")
	}
	l := &model.CultureLineage{
		ID:        id,
		Name:      name,
		Status:    model.LineageFiled,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.Store.SaveLineage(l); err != nil {
		return nil, err
	}
	return l, nil
}

// TransitionLineage validates and applies a lineage status transition.
func (s *Services) TransitionLineage(id string, to model.LineageStatus) (*model.CultureLineage, error) {
	l, err := s.Store.GetLineage(id)
	if err != nil {
		return nil, err
	}
	if l.Status == model.LineageSealed {
		return nil, model.ErrSealedMutate
	}
	if !model.ValidLineageTransition(l.Status, to) {
		return nil, fmt.Errorf("service: invalid lineage transition %s->%s", l.Status, to)
	}
	l.Status = to
	if to == model.LineageSealed {
		now := time.Now().UTC()
		l.SealedAt = &now
	}
	if err := s.Store.SaveLineage(l); err != nil {
		return nil, err
	}
	return l, nil
}

// IngestRead delegates to the read service.
func (s *Services) IngestRead(lineageID string, generation int, bc string, q []int) (*model.BarcodeRead, error) {
	return s.Read.IngestRead(lineageID, generation, bc, q)
}

// AddGenerationEdge validates (cycle guard) then stores an edge.
func (s *Services) AddGenerationEdge(e model.GenerationEdge) error {
	if err := s.Lineage.ValidateEdge(e); err != nil {
		return err
	}
	return s.Read.AddGenerationEdge(e)
}

// CorrectGeneration runs correction/clustering for one generation.
func (s *Services) CorrectGeneration(lineageID string, generation int) ([]*model.CorrectionCluster, error) {
	return s.Correction.ClusterGeneration(lineageID, generation)
}

// LockAncestor delegates to lineage.
func (s *Services) LockAncestor(lineageID string, generation int) error {
	return s.Lineage.LockAncestor(lineageID, generation)
}

// AnalyzeGeneration runs contamination analysis for one generation.
func (s *Services) AnalyzeGeneration(lineageID string, generation int) ([]*model.ContaminationCandidate, error) {
	return s.Discriminate.AnalyzeGeneration(lineageID, generation)
}

// DecideCandidate delegates to discriminate.
func (s *Services) DecideCandidate(id string, confirm bool, note string) error {
	return s.Discriminate.DecideCandidate(id, confirm, note)
}

// PublishSnapshot delegates to snapshot.
func (s *Services) PublishSnapshot(lineageID, summary string) (*model.DiscriminationSnapshot, error) {
	return s.Snapshot.Publish(lineageID, summary)
}

// ConfirmSnapshot delegates to snapshot.
func (s *Services) ConfirmSnapshot(id string) error {
	return s.Snapshot.Confirm(id)
}

// SelfCheck runs a lightweight integrity report over the database.
func (s *Services) SelfCheck() (map[string]int, error) {
	lineages, err := s.Store.ListLineages()
	if err != nil {
		return nil, err
	}
	counts := map[string]int{"lineages": len(lineages)}
	for _, l := range lineages {
		reads, err := s.Store.ListReadsByLineage(l.ID)
		if err != nil {
			return nil, err
		}
		clusters, err := s.Store.ListClusters(l.ID)
		if err != nil {
			return nil, err
		}
		cands, err := s.Store.ListCandidates(l.ID)
		if err != nil {
			return nil, err
		}
		counts["reads"] += len(reads)
		counts["clusters"] += len(clusters)
		counts["candidates"] += len(cands)
	}
	return counts, nil
}
