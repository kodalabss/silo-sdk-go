package silo

import (
	"crypto/sha256"
	"encoding/binary"
	"math/big"
)

const (
	// P is a Mersenne prime (2^61 - 1)
	LCT_P = 2305843009213693951
)

type LCTState struct {
	P     uint64
	S     uint64
	index int
}

func NewLCTState(seed []byte) *LCTState {
	h := sha256.Sum256(seed)
	s := binary.BigEndian.Uint64(h[:8])
	return &LCTState{
		P:     LCT_P,
		S:     s % LCT_P,
		index: 0,
	}
}

// Evolve advances the state: Si+1 = (Si^3 + 13Si + 7i^2 + Sum(k+1)bk) mod P
func (st *LCTState) Evolve(block []byte) {
	si := new(big.Int).SetUint64(st.S)
	p := new(big.Int).SetUint64(st.P)

	term1 := new(big.Int).Exp(si, big.NewInt(3), p)

	term2 := new(big.Int).Mul(big.NewInt(13), si)
	term2.Mod(term2, p)

	idx := big.NewInt(int64(st.index))
	term3 := new(big.Int).Mul(big.NewInt(7), new(big.Int).Mul(idx, idx))
	term3.Mod(term3, p)

	sum := big.NewInt(0)
	for k, b := range block {
		bk := big.NewInt(int64(b))
		coeff := big.NewInt(int64(k + 1))
		sum.Add(sum, new(big.Int).Mul(coeff, bk))
	}
	sum.Mod(sum, p)

	res := new(big.Int).Add(term1, term2)
	res.Add(res, term3)
	res.Add(res, sum)
	res.Mod(res, p)

	st.S = res.Uint64()
	st.index++
}

// Project transforms a block of 3 bytes into a vector (x, y, z)
func (st *LCTState) Project(b []byte) (x, y, z uint64) {
	var b0, b1, b2 uint64
	if len(b) > 0 { b0 = uint64(b[0]) }
	if len(b) > 1 { b1 = uint64(b[1]) }
	if len(b) > 2 { b2 = uint64(b[2]) }

	x = (b0 + st.S) % st.P
	y = (b1*b1 + 5*st.S) % st.P
	z = (3*b2 + 7*y + st.S) % st.P
	return
}

// Mix applies the state-derived matrix M
func (st *LCTState) Mix(x, y, z uint64) (v1, v2, v3 uint64) {
	s := st.S
	v1 = ((2+s)*x + 5*y + 7*z) % st.P
	v2 = (11*x + (3+s)*y + 13*z) % st.P
	v3 = (17*x + 19*y + (23+s)*z) % st.P
	return
}

// LCTEncode transforms plaintext bytes into a sequence of LCT vectors
func LCTEncode(data []byte, seed []byte) []uint64 {
	st := NewLCTState(seed)
	var output []uint64

	for i := 0; i < len(data); i += 3 {
		end := i + 3
		if end > len(data) {
			end = len(data)
		}
		block := data[i:end]

		x, y, z := st.Project(block)
		v1, v2, v3 := st.Mix(x, y, z)
		output = append(output, v1, v2, v3)

		st.Evolve(block)
	}

	return output
}
