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

// Resolve maps a path to a deterministic numeric coordinate.
func Resolve(workspaceID string, path string, signatures map[string]string) uint64 {
	hFinal := H(fmt.Sprintf("%s:0", workspaceID))
	if path == "" {
		return hFinal
	}
	segments := strings.Split(path, "/")
	segmentName := segments[0]

	if segmentName == "__proj__" && len(segments) == 4 {
		floorPath := strings.Join(segments[:3], "/")
		lCoord := uint64(0)
		fmt.Sscanf(segments[3], "%d", &lCoord)
		hFloor := Resolve(workspaceID, floorPath, nil)
		return (hFloor & ^uint64(1023)) | (lCoord & 1023)
	}

	for i, segment := range segments {
		pos := i + 1
		input := fmt.Sprintf("%s:%d", segment, pos)

		if i == 2 {
			key := fmt.Sprintf("%s:%s", segmentName, segment)
			if sig, ok := signatures[key]; ok {
				input = fmt.Sprintf("%s:%s:%d", segment, sig, pos)
			}
		}

		hFinal ^= H(input)
	}
	return hFinal
}

// DeriveS0 creates the root secret from the workspace key.
func (s *Silo) DeriveS0() []byte {
	h := sha256.New()
	h.Write([]byte(s.Token))
	return h.Sum(nil)
}

// DeriveSn evolves the root secret into a session secret.
func (s *Silo) DeriveSn(epoch int64) []byte {
	s0 := s.DeriveS0()
	h := sha256.New()
	h.Write(s0)
	h.Write([]byte(fmt.Sprintf("%d", epoch)))
	return h.Sum(nil)
}

// GenerateProof creates a unified access proof for a request.
// HMAC signature formula: HMAC(Sn, G || Epoch || Nonce || Seq || ReqHash)
func (s *Silo) GenerateProof(path string, reqHash string, nonce string, sequenceID string, epoch int64) string {
	s.mu.RLock()
	sigs := s.signatures
	wsID := s.wsID
	s.mu.RUnlock()

	if wsID == "" {
		return ""
	}

	sn := s.DeriveSn(epoch)
	g := Resolve(wsID, path, sigs)

	// HMAC generation
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
	clean := bytes.TrimSpace(data)
	if len(clean) == 0 {
		return ""
	}
	h := sha256.New()
	h.Write(clean)
	return hex.EncodeToString(h.Sum(nil))
}

// StableMarshal produces a deterministic JSON string.
func StableMarshal(v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return bytes.TrimSpace(data), nil
}
