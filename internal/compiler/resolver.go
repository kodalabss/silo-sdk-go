package compiler

import (
	"github.com/cespare/xxhash/v2"
)

type ResolvedManifest struct {
	Version    int
	Signatures map[string]uint64
	Dimensions []Dimension
}

type Dimension struct {
	Name    string
	Intents []ResolvedIntent
}

type ResolvedIntent struct {
	Type   string
	Params map[string]interface{}
}

func Resolve(m *Manifest) *ResolvedManifest {
	rm := &ResolvedManifest{
		Version:    m.Version,
		Signatures: make(map[string]uint64),
	}

	for _, entry := range m.Entries {
		walk(entry, "", rm)
	}

	return rm
}

func walk(e Entry, parentPath string, rm *ResolvedManifest) {
	currentPath := e.Name
	if parentPath != "" {
		currentPath = parentPath + "/" + e.Name
	}

	// 1. Resolve Geometry Signature (XOR-Hash)
	// For v0.1, we store the hash of the path itself as the signature.
	if !e.IsVariable {
		rm.Signatures[currentPath] = xxhash.Sum64([]byte(currentPath))
	}

	// 2. Build Dimensions (Excluding variables from names)
	if len(e.Intents) > 0 {
		dimName := e.Name
		// In a real implementation, we would trace the path back up
		// skipping [variable] nodes to create the dot-notation name.
		dim := Dimension{Name: dimName}
		for _, intent := range e.Intents {
			dim.Intents = append(dim.Intents, ResolvedIntent{
				Type:   intent.Name,
				Params: intent.Params,
			})
		}
		rm.Dimensions = append(rm.Dimensions, dim)
	}

	for _, child := range e.Children {
		walk(child, currentPath, rm)
	}
}
