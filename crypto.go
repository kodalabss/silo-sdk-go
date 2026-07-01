package silo

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/cespare/xxhash/v2"
	"strings"
)

// H computes a hash of the input string.
func H(input string) uint64 {
	return xxhash.Sum64([]byte(input))
}

// Resolve maps a path to a numeric coordinate.
func Resolve(workspaceID string, path string, signatures map[string]string) uint64 {
	hFinal := H(fmt.Sprintf("%s:0", workspaceID))
	if path == "" {
		return hFinal
	}
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		pos := i + 1
		input := fmt.Sprintf("%s:%d", segment, pos)

		if sig, ok := signatures[segment]; ok {
			input = fmt.Sprintf("%s:%s:%d", segment, sig, pos)
		}

		hFinal ^= H(input)
	}
	return hFinal
}

// GenerateProof creates a temporary access proof for a request.
func (s *Silo) GenerateProof(path string, epoch int64) string {
	s.mu.RLock()
	snHex := s.sn
	storedEpoch := s.epoch
	sigs := s.signatures
	s.mu.RUnlock()

	if snHex == "" || epoch != storedEpoch {
		return ""
	}

	sn, _ := hex.DecodeString(snHex)

	parts := strings.Split(strings.TrimPrefix(s.Token, "koda_wk_"), "_")
	if len(parts) < 1 {
		return ""
	}
	wsID := parts[0]

	g := Resolve(wsID, path, sigs)

	mac := hmac.New(sha256.New, sn)
	payload := fmt.Sprintf("%d:%d", g, epoch)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
