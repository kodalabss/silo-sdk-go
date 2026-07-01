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

// Project transforms a single byte into a coordinate (x, y, z)
func (st *LCTState) Project(b byte) (x, y, z uint64) {
	x = (uint64(b) + st.S) % st.P

	// y = (x^2 + 5s) mod P
	xx := new(big.Int).SetUint64(x)
	p := new(big.Int).SetUint64(st.P)
	yVal := new(big.Int).Mul(xx, xx)
	yVal.Add(yVal, new(big.Int).Mul(big.NewInt(5), new(big.Int).SetUint64(st.S)))
	yVal.Mod(yVal, p)
	y = yVal.Uint64()

	// z = (3x + 7y + s) mod P
	zVal := new(big.Int).Mul(big.NewInt(3), xx)
	zVal.Add(zVal, new(big.Int).Mul(big.NewInt(7), yVal))
	zVal.Add(zVal, new(big.Int).SetUint64(st.S))
	zVal.Mod(zVal, p)
	z = zVal.Uint64()

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

// Unmix solves M*[x,y,z] = [v1,v2,v3] mod P
func (st *LCTState) Unmix(v1, v2, v3 uint64) (x, y, z uint64) {
	s := new(big.Int).SetUint64(st.S)
	p := new(big.Int).SetUint64(st.P)

	a := new(big.Int).Add(big.NewInt(2), s)
	b := big.NewInt(5)
	c := big.NewInt(7)
	d := big.NewInt(11)
	e := new(big.Int).Add(big.NewInt(3), s)
	f := big.NewInt(13)
	g := big.NewInt(17)
	h := big.NewInt(19)
	i := new(big.Int).Add(big.NewInt(23), s)

	// Adjugate matrix entries (cofactors)
	ei_fh := new(big.Int).Sub(new(big.Int).Mul(e, i), new(big.Int).Mul(f, h))
	di_fg := new(big.Int).Sub(new(big.Int).Mul(d, i), new(big.Int).Mul(f, g))
	dh_eg := new(big.Int).Sub(new(big.Int).Mul(d, h), new(big.Int).Mul(e, g))

	det := new(big.Int).Mul(a, ei_fh)
	det.Sub(det, new(big.Int).Mul(b, di_fg))
	det.Add(det, new(big.Int).Mul(c, dh_eg))
	det.Mod(det, p)

	detInv := new(big.Int).ModInverse(det, p)

	A := ei_fh
	B := new(big.Int).Neg(di_fg)
	C := dh_eg
	D := new(big.Int).Neg(new(big.Int).Sub(new(big.Int).Mul(b, i), new(big.Int).Mul(c, h)))
	E := new(big.Int).Sub(new(big.Int).Mul(a, i), new(big.Int).Mul(c, g))
	F := new(big.Int).Neg(new(big.Int).Sub(new(big.Int).Mul(a, h), new(big.Int).Mul(b, g)))
	G := new(big.Int).Sub(new(big.Int).Mul(b, f), new(big.Int).Mul(c, e))
	H := new(big.Int).Neg(new(big.Int).Sub(new(big.Int).Mul(a, f), new(big.Int).Mul(c, d)))
	I := new(big.Int).Sub(new(big.Int).Mul(a, e), new(big.Int).Mul(b, d))

	vv1 := new(big.Int).SetUint64(v1)
	vv2 := new(big.Int).SetUint64(v2)
	vv3 := new(big.Int).SetUint64(v3)

	rx := new(big.Int).Mul(A, vv1)
	rx.Add(rx, new(big.Int).Mul(D, vv2))
	rx.Add(rx, new(big.Int).Mul(G, vv3))
	rx.Mul(rx, detInv)
	rx.Mod(rx, p)

	ry := new(big.Int).Mul(B, vv1)
	ry.Add(ry, new(big.Int).Mul(E, vv2))
	ry.Add(ry, new(big.Int).Mul(H, vv3))
	ry.Mul(ry, detInv)
	ry.Mod(ry, p)

	rz := new(big.Int).Mul(C, vv1)
	rz.Add(rz, new(big.Int).Mul(F, vv2))
	rz.Add(rz, new(big.Int).Mul(I, vv3))
	rz.Mul(rz, detInv)
	rz.Mod(rz, p)

	return rx.Uint64(), ry.Uint64(), rz.Uint64()
}

func LCTEncode(data []byte, seed []byte) []uint64 {
	st := NewLCTState(seed)
	var output []uint64

	for _, b := range data {
		x, y, z := st.Project(b)
		v1, v2, v3 := st.Mix(x, y, z)
		output = append(output, v1, v2, v3)
		st.Evolve([]byte{b})
	}

	return output
}

func LCTDecode(vectors []uint64, seed []byte) []byte {
	st := NewLCTState(seed)
	var output []byte

	for i := 0; i < len(vectors); i += 3 {
		v1, v2, v3 := vectors[i], vectors[i+1], vectors[i+2]
		x, _, _ := st.Unmix(v1, v2, v3)

		// b = (x - s) mod P
		b := (x + st.P - (st.S % st.P)) % st.P
		val := byte(b & 0xFF)
		output = append(output, val)

		st.Evolve([]byte{val})
	}

	return output
}
