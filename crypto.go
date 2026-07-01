package silo

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/cespare/xxhash/v2"
	"strings"
)

// H computes the xxHash64 of the input string.
// This is public/standard math, safe to show.
func H(input string) uint64 {
	return xxhash.Sum64([]byte(input))
}

// Resolve maps a workspace and a path to a 64-bit coordinate.
// This is public geometry logic, safe to show.
func Resolve(workspaceID string, path string) uint64 {
	hFinal := H(fmt.Sprintf("%s:0", workspaceID))
	if path == "" {
		return hFinal
	}
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		pos := i + 1
		hFinal ^= H(fmt.Sprintf("%s:%d", segment, pos))
	}
	return hFinal
}

// GenerateProof creates an ephemeral security proof for a coordinate.
// This is a "Black Box" operation using the Sn provided by the server.
// The SDK does NOT know how Sn is derived.
func (s *Silo) GenerateProof(path string, epoch int64) string {
	s.mu.RLock()
	snHex := s.sn
	storedEpoch := s.epoch
	s.mu.RUnlock()

	// If we don't have a valid Sn for this epoch, we can't sign.
	// The client should have called Handshake().
	if snHex == "" || epoch != storedEpoch {
		return ""
	}

	sn, _ := hex.DecodeString(snHex)

	// 1. Extract Workspace ID from token
	parts := strings.Split(strings.TrimPrefix(s.Token, "koda_wk_"), "_")
	if len(parts) < 1 {
		return ""
	}
	wsID := parts[0]

	// 2. Resolve Geometry
	g := Resolve(wsID, path)

	// 3. Derive P = HMAC(Sn, G || epoch)
	mac := hmac.New(sha256.New, sn)
	payload := fmt.Sprintf("%d:%d", g, epoch)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
