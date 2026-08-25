// Package model defines the domain entities, state machines and errors for the
// yeast barcode pedigree contamination discriminator.
package model

import (
	"errors"
	"time"
)

// BarcodeAlphabet is the allowed character set for a yeast barcode read.
// A barcode is a fixed alphabet over the nucleotide-like symbols A/C/G/T plus
// ambiguity codes N (any) and the separator '-' used between dual barcodes.
const BarcodeAlphabet = "ACGTN-"

// ErrBarcodeIllegal is returned when a barcode string uses an illegal character.
var ErrBarcodeIllegal = errors.New("model: illegal barcode character")

// ErrGenerationCycle is returned when a generation edge would create a cycle.
var ErrGenerationCycle = errors.New("model: generation cycle detected")

// ErrQualityMissing is returned when a read has no quality scores.
var ErrQualityMissing = errors.New("model: read quality scores missing")

// ErrSealedMutate is returned when a sealed entity is mutated.
var ErrSealedMutate = errors.New("model: sealed entity cannot be mutated")

// ErrNotFound is returned when an entity does not exist.
var ErrNotFound = errors.New("model: entity not found")

// ErrDuplicate is returned when a unique constraint is violated.
var ErrDuplicate = errors.New("model: duplicate entity")

// ErrStateConflict is returned when a concurrent state transition lost a
// compare-and-set race against another request.
var ErrStateConflict = errors.New("model: concurrent state transition")

// LineageStatus enumerates the lifecycle of a culture lineage.
type LineageStatus string

const (
	LineageFiled      LineageStatus = "filed"      // 建档
	LineageSequencing LineageStatus = "sequencing" // 测序中
	LineagePending    LineageStatus = "pending"    // 待判别
	LineageConfirmed  LineageStatus = "confirmed"  // 确认
	LineageSealed     LineageStatus = "sealed"     // 封存
)

// ReadStatus enumerates the lifecycle of a barcode read.
type ReadStatus string

const (
	ReadRaw        ReadStatus = "raw"         // 原始
	ReadCorrected  ReadStatus = "corrected"   // 已纠错
	ReadLowQuality ReadStatus = "low_quality" // 低质
	ReadDuplicate  ReadStatus = "duplicate"   // 重复
	ReadExcluded   ReadStatus = "excluded"    // 排除
)

// CandidateStatus enumerates the lifecycle of a contamination candidate.
type CandidateStatus string

const (
	CandGenerated    CandidateStatus = "generated"    // 生成
	CandInsufficient CandidateStatus = "insufficient" // 证据不足
	CandConfirmed    CandidateStatus = "confirmed"    // 确认
	CandRejected     CandidateStatus = "rejected"     // 否决
)

// SnapshotStatus enumerates the lifecycle of a discrimination snapshot.
type SnapshotStatus string

const (
	SnapDraft      SnapshotStatus = "draft"      // 草稿
	SnapPublished  SnapshotStatus = "published"  // 发布
	SnapSuperseded SnapshotStatus = "superseded" // 替代
)

// CultureLineage is a bacterial/yeast culture tracked across generations.
type CultureLineage struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Status    LineageStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	SealedAt  *time.Time    `json:"sealed_at,omitempty"`
}

// BarcodeRead is one sequencing read of a barcode from a culture generation.
type BarcodeRead struct {
	ID               string     `json:"id"`
	LineageID        string     `json:"lineage_id"`
	Generation       int        `json:"generation"`
	RawBarcode       string     `json:"raw_barcode"`
	CorrectedBarcode string     `json:"corrected_barcode,omitempty"`
	Quality          []int      `json:"quality"` // phred-like per position
	Status           ReadStatus `json:"status"`
	ClusterID        string     `json:"cluster_id,omitempty"`
	Hash             string     `json:"hash"` // content hash for idempotency
	CreatedAt        time.Time  `json:"created_at"`
}

// GenerationEdge is a parent->child relationship between lineage generations.
type GenerationEdge struct {
	ParentGeneration int    `json:"parent_generation"`
	ChildGeneration  int    `json:"child_generation"`
	LineageID        string `json:"lineage_id"`
}

// CorrectionCluster groups reads that collapse to the same corrected barcode.
type CorrectionCluster struct {
	ID               string `json:"id"`
	LineageID        string `json:"lineage_id"`
	Generation       int    `json:"generation"`
	CanonicalBarcode string `json:"canonical_barcode"`
	ReadCount        int    `json:"read_count"`
	IsAncestor       bool   `json:"is_ancestor"` // locked ancestor clone barcode
}

// ContaminationCandidate is a foreign barcode suspected of contamination.
type ContaminationCandidate struct {
	ID            string          `json:"id"`
	LineageID     string          `json:"lineage_id"`
	Generation    int             `json:"generation"`
	Barcode       string          `json:"barcode"`
	EvidenceScore float64         `json:"evidence_score"`
	Frequency     float64         `json:"frequency"`
	Source        string          `json:"source"` // how it was detected
	Status        CandidateStatus `json:"status"`
	VerdictNote   string          `json:"verdict_note,omitempty"`
	DecidedAt     *time.Time      `json:"decided_at,omitempty"`
}

// DiscriminationSnapshot freezes a discrimination result for a lineage.
type DiscriminationSnapshot struct {
	ID           string         `json:"id"`
	LineageID    string         `json:"lineage_id"`
	Status       SnapshotStatus `json:"status"`
	Summary      string         `json:"summary"`
	ResultJSON   string         `json:"result_json"`
	CreatedAt    time.Time      `json:"created_at"`
	SupersededBy string         `json:"superseded_by,omitempty"`
}

// ValidLineageTransition checks the allowed status moves for a culture lineage.
func ValidLineageTransition(from, to LineageStatus) bool {
	switch from {
	case LineageFiled:
		return to == LineageSequencing || to == LineagePending || to == LineageSealed
	case LineageSequencing:
		return to == LineagePending || to == LineageSealed
	case LineagePending:
		return to == LineageConfirmed || to == LineageSealed
	case LineageConfirmed:
		return to == LineageSealed
	case LineageSealed:
		return false // terminal
	}
	return false
}

// ValidCandidateTransition checks allowed status moves for a candidate.
func ValidCandidateTransition(from, to CandidateStatus) bool {
	switch from {
	case CandGenerated:
		return to == CandInsufficient || to == CandConfirmed || to == CandRejected
	case CandInsufficient:
		return to == CandConfirmed || to == CandRejected
	case CandConfirmed, CandRejected:
		return false // terminal
	}
	return false
}

// ValidSnapshotTransition checks allowed status moves for a snapshot.
func ValidSnapshotTransition(from, to SnapshotStatus) bool {
	switch from {
	case SnapDraft:
		return to == SnapPublished || to == SnapSuperseded
	case SnapPublished:
		return to == SnapSuperseded
	case SnapSuperseded:
		return false
	}
	return false
}
