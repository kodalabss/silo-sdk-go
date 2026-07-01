package silo

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math/big"
)

const LCT_P_STR = "2305843009213693951"

var (
	ErrLCTCorruption = errors.New("LCT_SUBSTANCE_CORRUPTION_ERROR: Reality parity mismatch")
)

type LCTState struct {
	P *big.Int
	S *big.Int
	i int64
}

func NewLCTState(seed []byte) *LCTState {
	h := sha256.Sum256(seed)
	sVal := new(big.Int).SetUint64(binary.BigEndian.Uint64(h[:8]))
	p, _ := new(big.Int).SetString(LCT_P_STR, 10)
	return &LCTState{
		P: p,
		S: new(big.Int).Mod(sVal, p),
		i: 0,
	}
}

func (st *LCTState) Evolve(b byte) {
	p := st.P
	s := st.S
	s3 := new(big.Int).Exp(s, big.NewInt(3), p)
	s13 := new(big.Int).Mul(big.NewInt(13), s)
	s13.Mod(s13, p)
	iBig := new(big.Int).SetInt64(st.i)
	i2 := new(big.Int).Mul(iBig, iBig)
	i2.Mul(i2, big.NewInt(7)).Mod(i2, p)
	bv := big.NewInt(int64(b) + 1)
	newS := new(big.Int).Add(s3, s13)
	newS.Add(newS, i2).Add(newS, bv).Mod(newS, p)
	st.S = newS
	st.i++
}

func (st *LCTState) Mix(b byte) (v1, v2, v3 uint64) {
	p := st.P
	s := st.S

	// Project x, y, z
	x := new(big.Int).Add(new(big.Int).SetUint64(uint64(b)), s)
	x.Mod(x, p)

	y := new(big.Int).Mul(x, x)
	y.Add(y, new(big.Int).Mul(big.NewInt(5), s)).Mod(y, p)

	z := new(big.Int).Mul(big.NewInt(3), x)
	z.Add(z, new(big.Int).Mul(big.NewInt(7), y)).Add(z, s).Mod(z, p)

	a := new(big.Int).Add(big.NewInt(2), s)
	bVal := big.NewInt(5)
	c := big.NewInt(7)
	d := big.NewInt(11)
	e := new(big.Int).Add(big.NewInt(3), s)
	f := big.NewInt(13)
	g := big.NewInt(17)
	h := big.NewInt(19)
	i := new(big.Int).Add(big.NewInt(23), s)

	vv1 := new(big.Int).Mul(a, x)
	vv1.Add(vv1, new(big.Int).Mul(bVal, y)).Add(vv1, new(big.Int).Mul(c, z)).Mod(vv1, p)

	vv2 := new(big.Int).Mul(d, x)
	vv2.Add(vv2, new(big.Int).Mul(e, y)).Add(vv2, new(big.Int).Mul(f, z)).Mod(vv2, p)

	vv3 := new(big.Int).Mul(g, x)
	vv3.Add(vv3, new(big.Int).Mul(h, y)).Add(vv3, new(big.Int).Mul(i, z)).Mod(vv3, p)

	return vv1.Uint64(), vv2.Uint64(), vv3.Uint64()
}

func (st *LCTState) Unmix(v1, v2, v3 uint64) (byte, error) {
	p := st.P
	s := st.S
	a := new(big.Int).Add(big.NewInt(2), s)
	bVal := big.NewInt(5)
	c := big.NewInt(7)
	d := big.NewInt(11)
	e := new(big.Int).Add(big.NewInt(3), s)
	f := big.NewInt(13)
	g := big.NewInt(17)
	h := big.NewInt(19)
	i := new(big.Int).Add(big.NewInt(23), s)

	// det = a(ei - fh) - b(di - fg) + c(dh - eg)
	C11 := new(big.Int).Sub(new(big.Int).Mul(e, i), new(big.Int).Mul(f, h))
	C12 := new(big.Int).Sub(new(big.Int).Mul(d, i), new(big.Int).Mul(f, g)); C12.Neg(C12)
	C13 := new(big.Int).Sub(new(big.Int).Mul(d, h), new(big.Int).Mul(e, g))

	det := new(big.Int).Mul(a, C11)
	det.Add(det, new(big.Int).Mul(bVal, C12)).Add(det, new(big.Int).Mul(c, C13)).Mod(det, p)
	detInv := new(big.Int).ModInverse(det, p)

	// Adjugate for x, y, z
	// Row 1 (for x): C11, - (bi - ch), (bf - ce)
	adj11 := C11
	adj12 := new(big.Int).Sub(new(big.Int).Mul(bVal, i), new(big.Int).Mul(c, h)); adj12.Neg(adj12)
	adj13 := new(big.Int).Sub(new(big.Int).Mul(bVal, f), new(big.Int).Mul(c, e))

	// Row 2 (for y): - (di - fg), (ai - cg), - (af - cd)
	adj21 := C12
	adj22 := new(big.Int).Sub(new(big.Int).Mul(a, i), new(big.Int).Mul(c, g))
	adj23 := new(big.Int).Sub(new(big.Int).Mul(a, f), new(big.Int).Mul(c, d)); adj23.Neg(adj23)

	// Row 3 (for z): (dh - eg), - (ah - bg), (ae - bd)
	adj31 := C13
	adj32 := new(big.Int).Sub(new(big.Int).Mul(a, h), new(big.Int).Mul(bVal, g)); adj32.Neg(adj32)
	adj33 := new(big.Int).Sub(new(big.Int).Mul(a, e), new(big.Int).Mul(bVal, d))

	vv1 := new(big.Int).SetUint64(v1)
	vv2 := new(big.Int).SetUint64(v2)
	vv3 := new(big.Int).SetUint64(v3)

	rx := new(big.Int).Mul(adj11, vv1); rx.Add(rx, new(big.Int).Mul(adj12, vv2)).Add(rx, new(big.Int).Mul(adj13, vv3)); rx.Mul(rx, detInv).Mod(rx, p)
	ry := new(big.Int).Mul(adj21, vv1); ry.Add(ry, new(big.Int).Mul(adj22, vv2)).Add(ry, new(big.Int).Mul(adj23, vv3)); ry.Mul(ry, detInv).Mod(ry, p)
	rz := new(big.Int).Mul(adj31, vv1); rz.Add(rz, new(big.Int).Mul(adj32, vv2)).Add(rz, new(big.Int).Mul(adj33, vv3)); rz.Mul(rz, detInv).Mod(rz, p)

	// --- REALITY PARITY CHECK ---
	// 1. Recover b from x
	bVal := new(big.Int).Sub(rx, s); bVal.Add(bVal, p).Mod(bVal, p)
	b := byte(bVal.Uint64() & 0xFF)

	// 2. Re-project y and z from rx
	expectedY := new(big.Int).Mul(rx, rx)
	expectedY.Add(expectedY, new(big.Int).Mul(big.NewInt(5), s)).Mod(expectedY, p)

	expectedZ := new(big.Int).Mul(big.NewInt(3), rx)
	expectedZ.Add(expectedZ, new(big.Int).Mul(big.NewInt(7), expectedY)).Add(expectedZ, s).Mod(expectedZ, p)

	if ry.Cmp(expectedY) != 0 || rz.Cmp(expectedZ) != 0 {
		return 0, ErrLCTCorruption
	}

	return b, nil
}

func LCTPack(data []byte, seed []byte) string {
	st := NewLCTState(seed)
	buf := make([]byte, len(data)*24)
	for i, b := range data {
		v1, v2, v3 := st.Mix(b)
		binary.BigEndian.PutUint64(buf[i*24:], v1)
		binary.BigEndian.PutUint64(buf[i*24+8:], v2)
		binary.BigEndian.PutUint64(buf[i*24+16:], v3)
		st.Evolve(b)
	}
	return hex.EncodeToString(buf)
}

func LCTUnpack(hexData string, seed []byte) ([]byte, error) {
	buf, err := hex.DecodeString(hexData)
	if err != nil || len(buf)%24 != 0 {
		return nil, errors.New("invalid substance format")
	}
	st := NewLCTState(seed)
	var output []byte
	for i := 0; i < len(buf); i += 24 {
		v1 := binary.BigEndian.Uint64(buf[i:])
		v2 := binary.BigEndian.Uint64(buf[i+8:])
		v3 := binary.BigEndian.Uint64(buf[i+16:])
		b, err := st.Unmix(v1, v2, v3)
		if err != nil {
			return nil, err
		}
		output = append(output, b)
		st.Evolve(b)
	}
	return output, nil
}
