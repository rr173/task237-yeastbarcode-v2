// Package lineage maintains the generation inheritance graph of a culture: it
// connects parent and child generations through stored edges, detects illegal
// cycles and resolves, for each generation, the set of barcodes that are
// expected by inheritance (the "expected" barcode set).
package lineage

import (
	"fmt"

	"task237-yeastbarcode/internal/model"
	"task237-yeastbarcode/internal/store"
)

// Service resolves generation inheritance.
type Service struct {
	st *store.Store
}

// New constructs a lineage Service.
func New(st *store.Store) *Service { return &Service{st: st} }

// DetectCycle returns ErrGenerationCycle if adding edge would create a cycle in
// the lineage's generation graph (transitive reachability check).
func (s *Service) DetectCycle(lineageID string, edge model.GenerationEdge) error {
	edges, err := s.st.ListGenerationEdges(lineageID)
	if err != nil {
		return err
	}
	// build adjacency parent->children
	adj := map[int][]int{}
	for _, e := range edges {
		adj[e.ParentGeneration] = append(adj[e.ParentGeneration], e.ChildGeneration)
	}
	if reachable(adj, edge.ParentGeneration, edge.ChildGeneration) {
		return model.ErrGenerationCycle
	}
	radj := map[int][]int{}
	for _, e := range edges {
		radj[e.ChildGeneration] = append(radj[e.ChildGeneration], e.ParentGeneration)
	}
	if reachable(radj, edge.ChildGeneration, edge.ParentGeneration) {
		return model.ErrGenerationCycle
	}
	return nil
}

func reachable(adj map[int][]int, from, target int) bool {
	if from == target {
		return true
	}
	seen := map[int]bool{}
	stack := []int{from}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[n] {
			continue
		}
		seen[n] = true
		for _, c := range adj[n] {
			if c == target {
				return true
			}
			stack = append(stack, c)
		}
	}
	return false
}

// ExpectedBarcodes returns the canonical barcodes expected at a generation by
// inheritance from its direct parents. A generation with no parents (the
// founding generation) has no inherited expectation.
func (s *Service) ExpectedBarcodes(lineageID string, generation int) (map[string]bool, error) {
	edges, err := s.st.ListGenerationEdges(lineageID)
	if err != nil {
		return nil, err
	}
	parents := []int{}
	for _, e := range edges {
		if e.ChildGeneration == generation {
			parents = append(parents, e.ParentGeneration)
		}
	}
	expected := map[string]bool{}
	for _, p := range parents {
		clusters, err := s.st.ListClusters(lineageID)
		if err != nil {
			return nil, err
		}
		for _, c := range clusters {
			if c.Generation == p {
				expected[c.CanonicalBarcode] = true
			}
		}
	}
	return expected, nil
}

// ValidateEdge detects a cycle before delegating to the read service to persist.
func (s *Service) ValidateEdge(edge model.GenerationEdge) error {
	if err := model.ValidateGenerationEdge(edge); err != nil {
		return err
	}
	if err := s.DetectCycle(edge.LineageID, edge); err != nil {
		return fmt.Errorf("lineage: %w", err)
	}
	return nil
}
