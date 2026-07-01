package silo

import (
	"crypto/sha256"
	"encoding/binary"
	"math/big"
)

const (
	LCT_P = 2305843009213693951
)

type LCTState struct {
	P     *big.Int
	S     *big.Int
	index int
}

func NewLCTState(seed []byte) *LCTState {
	h := sha256.Sum256(seed)
	s := new(big.Int).SetUint64(binary.BigEndian.Uint64(h[:8]))
	p := new(big.Int).SetUint64(LCT_P)
	return &LCTState{
		P:     p,
		S:     new(big.Int).Mod(s, p),
		index: 0,
	}
}

func (st *LCTState) Evolve(block []byte) {
	term1 := new(big.Int).Exp(st.S, big.NewInt(3), st.P)
	term2 := new(big.Int).Mul(big.NewInt(13), st.S)
	term2.Mod(term2, st.P)

	idx := big.NewInt(int64(st.index))
	term3 := new(big.Int).Mul(big.NewInt(7), new(big.Int).Mul(idx, idx))
	term3.Mod(term3, st.P)

	sum := big.NewInt(0)
	for k, b := range block {
		bk := big.NewInt(int64(b))
		coeff := big.NewInt(int64(k + 1))
		sum.Add(sum, new(big.Int).Mul(coeff, bk))
	}
	sum.Mod(sum, st.P)

	st.S.Add(term1, term2)
	st.S.Add(st.S, term3)
	st.S.Add(st.S, sum)
	st.S.Mod(st.S, st.P)
	st.index++
}

func (st *LCTState) Project(b byte) (x, y, z *big.Int) {
	x = new(big.Int).Add(new(big.Int).SetUint64(uint64(b)), st.S)
	x.Mod(x, st.P)

	y = new(big.Int).Mul(x, x)
	y.Add(y, new(big.Int).Mul(big.NewInt(5), st.S))
	y.Mod(y, st.P)

	z = new(big.Int).Mul(big.NewInt(3), x)
	z.Add(z, new(big.Int).Mul(big.NewInt(7), y))
	z.Add(z, st.S)
	z.Mod(z, st.P)
	return
}

func (st *LCTState) Mix(x, y, z *big.Int) (v1, v2, v3 *big.Int) {
	// M = [ 2+s, 5, 7 ]
	//     [ 11, 3+s, 13 ]
	//     [ 17, 19, 23+s ]
	a := new(big.Int).Add(big.NewInt(2), st.S)
	e := new(big.Int).Add(big.NewInt(3), st.S)
	i := new(big.Int).Add(big.NewInt(23), st.S)

	v1 = new(big.Int).Mul(a, x)
	v1.Add(v1, new(big.Int).Mul(big.NewInt(5), y))
	v1.Add(v1, new(big.Int).Mul(big.NewInt(7), z))
	v1.Mod(v1, st.P)

	v2 = new(big.Int).Mul(big.NewInt(11), x)
	v2.Add(v2, new(big.Int).Mul(e, y))
	v2.Add(v2, new(big.Int).Mul(big.NewInt(13), z))
	v2.Mod(v2, st.P)

	v3 = new(big.Int).Mul(big.NewInt(17), x)
	v3.Add(v3, new(big.Int).Mul(big.NewInt(19), y))
	v3.Add(v3, new(big.Int).Mul(i, z))
	v3.Mod(v3, st.P)
	return
}

func (st *LCTState) Unmix(v1, v2, v3 *big.Int) (x, y, z *big.Int) {
	a := new(big.Int).Add(big.NewInt(2), st.S)
	b := big.NewInt(5)
	c := big.NewInt(7)
	d := big.NewInt(11)
	e := new(big.Int).Add(big.NewInt(3), st.S)
	f := big.NewInt(13)
	g := big.NewInt(17)
	h := big.NewInt(19)
	i := new(big.Int).Add(big.NewInt(23), st.S)

	// Cofactors
	C11 := new(big.Int).Sub(new(big.Int).Mul(e, i), new(big.Int).Mul(f, h))
	C12 := new(big.Int).Sub(new(big.Int).Mul(d, i), new(big.Int).Mul(f, g))
	C12.Neg(C12)
	C13 := new(big.Int).Sub(new(big.Int).Mul(d, h), new(big.Int).Mul(e, g))

	det := new(big.Int).Mul(a, C11)
	det.Add(det, new(big.Int).Mul(b, C12))
	det.Add(det, new(big.Int).Mul(c, C13))
	det.Mod(det, st.P)

	detInv := new(big.Int).ModInverse(det, st.P)

	C21 := new(big.Int).Sub(new(big.Int).Mul(b, i), new(big.Int).Mul(c, h))
	C21.Neg(C21)
	C22 := new(big.Int).Sub(new(big.Int).Mul(a, i), new(big.Int).Mul(c, g))
	C23 := new(big.Int).Sub(new(big.Int).Mul(a, h), new(big.Int).Mul(b, g))
	C23.Neg(C23)

	C31 := new(big.Int).Sub(new(big.Int).Mul(b, f), new(big.Int).Mul(c, e))
	C32 := new(big.Int).Sub(new(big.Int).Mul(a, f), new(big.Int).Mul(c, d))
	C32.Neg(C32)
	C33 := new(big.Int).Sub(new(big.Int).Mul(a, e), new(big.Int).Mul(b, d))

	// [x,y,z] = detInv * adj(M) * [v1,v2,v3]
	// adj(M) = transpose of cofactor matrix
	x = new(big.Int).Mul(C11, v1)
	x.Add(x, new(big.Int).Mul(C21, v2))
	x.Add(x, new(big.Int).Mul(C31, v3))
	x.Mul(x, detInv)
	x.Mod(x, st.P)

	y = new(big.Int).Mul(C12, v1)
	y.Add(y, new(big.Int).Mul(C22, v2))
	y.Add(y, new(big.Int).Mul(C32, v3))
	y.Mul(y, detInv)
	y.Mod(y, st.P)

	z = new(big.Int).Mul(C13, v1)
	z.Add(z, new(big.Int).Mul(C23, v2))
	z.Add(z, new(big.Int).Mul(C33, v3))
	z.Mul(z, detInv)
	z.Mod(z, st.P)
	return
}

func LCTEncode(data []byte, seed []byte) []uint64 {
	st := NewLCTState(seed)
	var output []uint64
	for _, b := range data {
		x, y, z := st.Project(b)
		v1, v2, v3 := st.Mix(x, y, z)
		output = append(output, v1.Uint64(), v2.Uint64(), v3.Uint64())
		st.Evolve([]byte{b})
	}
	return output
}

func LCTDecode(vectors []uint64, seed []byte) []byte {
	st := NewLCTState(seed)
	var output []byte
	for i := 0; i < len(vectors); i += 3 {
		v1 := new(big.Int).SetUint64(vectors[i])
		v2 := new(big.Int).SetUint64(vectors[i+1])
		v3 := new(big.Int).SetUint64(vectors[i+2])

		x, _, _ := st.Unmix(v1, v2, v3)

		// b = (x - s) mod P
		b := new(big.Int).Sub(x, st.S)
		b.Mod(b, st.P)

		val := byte(b.Uint64() & 0xFF)
		output = append(output, val)
		st.Evolve([]byte{val})
	}
	return output
}
