package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// ReadHash computes a stable content hash for a raw read so duplicate uploads
// are idempotent. It ignores timestamps and record IDs.
func ReadHash(lineageID string, generation int, rawBarcode string, quality []int) string {
	parts := []string{
		lineageID,
		fmt.Sprintf("g%d", generation),
		rawBarcode,
	}
	qs := make([]string, len(quality))
	for i, v := range quality {
		qs[i] = fmt.Sprintf("%d", v)
	}
	parts = append(parts, strings.Join(qs, ","))
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

// ClusterHash computes a stable hash for a correction cluster key.
func ClusterHash(lineageID string, generation int, canonical string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|g%d|%s", lineageID, generation, canonical)))
	return hex.EncodeToString(sum[:])
}

// SnapshotID computes a stable hash for one discrimination snapshot publish.
// It mixes the lineage, the reviewer summary, the frozen result payload and the
// publish timestamp so every publish — even two with identical content — gets a
// distinct identity and cannot collide with a prior publish (which would let a
// later publish overwrite an older published snapshot's summary and result).
func SnapshotID(lineageID, summary, resultJSON, createdAt string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s", lineageID, summary, resultJSON, createdAt)))
	return hex.EncodeToString(sum[:])
}

// SortedKeys returns the sorted keys of a string map (deterministic hashing).
func SortedKeys(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}
