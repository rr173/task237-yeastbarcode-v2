// Package read validates and ingests raw barcode reads into the store. It is
// the input boundary of the pipeline: illegal barcodes and missing quality are
// rejected before they can pollute the correction stage.
package read

import (
	"errors"
	"time"

	"task237-yeastbarcode/internal/model"
	"task237-yeastbarcode/internal/store"
)

// Service validates and stores barcode reads and generation edges.
type Service struct {
	st *store.Store
}

// New constructs a read Service.
func New(st *store.Store) *Service { return &Service{st: st} }

// IngestRead validates a raw read, computes its content hash for idempotency and
// stores it. It returns the stored read; if an identical read was already
// present it returns model.ErrDuplicate with the existing record attached.
// Dedup is decided atomically by the content-hash INSERT, so concurrent
// submissions of identical content yield exactly one stored read and report
// model.ErrDuplicate to every other caller.
func (s *Service) IngestRead(lineageID string, generation int, rawBarcode string, quality []int) (*model.BarcodeRead, error) {
	if err := s.st.EnsureLineageMutable(lineageID); err != nil {
		return nil, err
	}
	if err := model.ValidateBarcode(rawBarcode); err != nil {
		return nil, err
	}
	if err := model.ValidateQuality(rawBarcode, quality); err != nil {
		return nil, err
	}
	hash := model.ReadHash(lineageID, generation, rawBarcode, quality)
	r := &model.BarcodeRead{
		ID:         model.ClusterHash(lineageID, generation, hash)[:16],
		LineageID:  lineageID,
		Generation: generation,
		RawBarcode: rawBarcode,
		Quality:    quality,
		Status:     model.ReadRaw,
		Hash:       hash,
		CreatedAt:  time.Now().UTC(),
	}
	stored, err := s.st.SaveRead(r)
	if err != nil {
		if errors.Is(err, model.ErrDuplicate) {
			// idempotent: identical content already stored under the existing record
			return stored, model.ErrDuplicate
		}
		return nil, err
	}
	return stored, nil
}

// AddGenerationEdge validates and stores a parent->child generation edge. It
// rejects non-increasing generations (cycle guard at the edge level).
func (s *Service) AddGenerationEdge(e model.GenerationEdge) error {
	if err := s.st.EnsureLineageMutable(e.LineageID); err != nil {
		return err
	}
	if err := model.ValidateGenerationEdge(e); err != nil {
		return err
	}
	exists, err := s.st.GenerationEdgeExists(e)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return s.st.SaveGenerationEdge(e)
}
