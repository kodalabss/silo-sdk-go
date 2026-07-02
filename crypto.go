package silo

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

// GenerateProof creates a temporary access proof bound to the request.
func (s *Silo) GenerateProof(path string, reqHash string, nonce string, sequenceID string, epoch int64) string {
	s.mu.RLock()
	snHex := s.sn
	storedEpoch := s.epoch
	sigs := s.signatures
	wsID := s.wsID
	s.mu.RUnlock()

	if snHex == "" || epoch < storedEpoch-1 || wsID == "" {
		return ""
	}

	sn, _ := hex.DecodeString(snHex)
	g := Resolve(wsID, path, sigs)

	// Formula: P = HMAC(Sn, G || epoch || nonce || sequenceID || reqHash)
	mac := hmac.New(sha256.New, sn)
	payload := fmt.Sprintf("%d:%d:%s:%s:%s", g, epoch, nonce, sequenceID, reqHash)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// HashBody returns a hex sha256 of the data.
func HashBody(data []byte) string {
	if data == nil || len(data) == 0 {
		return ""
	}
	// Sanitize to match server-side TrimSpace
	clean := bytes.TrimSpace(data)
	if len(clean) == 0 {
		return ""
	}
	h := sha256.New()
	h.Write(clean)
	return hex.EncodeToString(h.Sum(nil))
}

// StableMarshal produces a deterministic JSON string by sorting keys.
func StableMarshal(v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil { return nil, err }
	return bytes.TrimSpace(data), nil
}
