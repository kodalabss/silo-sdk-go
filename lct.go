package silo

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math/big"
)

const LCT_P_STR = "2305843009213693951"

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
	x := new(big.Int).Add(new(big.Int).SetUint64(uint64(b)), s)
	x.Mod(x, p)
	y := big.NewInt(1)
	z := big.NewInt(2)
	a := new(big.Int).Add(big.NewInt(2), s)
	e := new(big.Int).Add(big.NewInt(3), s)
	i := new(big.Int).Add(big.NewInt(23), s)
	vv1 := new(big.Int).Mul(a, x)
	vv1.Add(vv1, new(big.Int).Mul(big.NewInt(5), y)).Add(vv1, new(big.Int).Mul(big.NewInt(7), z)).Mod(vv1, p)
	vv2 := new(big.Int).Mul(big.NewInt(11), x)
	vv2.Add(vv2, new(big.Int).Mul(e, y)).Add(vv2, new(big.Int).Mul(big.NewInt(13), z)).Mod(vv2, p)
	vv3 := new(big.Int).Mul(big.NewInt(17), x)
	vv3.Add(vv3, new(big.Int).Mul(big.NewInt(19), y)).Add(vv3, new(big.Int).Mul(i, z)).Mod(vv3, p)
	return vv1.Uint64(), vv2.Uint64(), vv3.Uint64()
}

func (st *LCTState) Unmix(v1, v2, v3 uint64) byte {
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
	ei_fh := new(big.Int).Sub(new(big.Int).Mul(e, i), new(big.Int).Mul(f, h))
	di_fg := new(big.Int).Sub(new(big.Int).Mul(d, i), new(big.Int).Mul(f, g))
	dh_eg := new(big.Int).Sub(new(big.Int).Mul(d, h), new(big.Int).Mul(e, g))
	det := new(big.Int).Mul(a, ei_fh)
	det.Sub(det, new(big.Int).Mul(bVal, di_fg))
	det.Add(det, new(big.Int).Mul(c, dh_eg))
	det.Mod(det, p)
	detInv := new(big.Int).ModInverse(det, p)
	C11 := ei_fh
	C21 := new(big.Int).Sub(new(big.Int).Mul(bVal, i), new(big.Int).Mul(c, h))
	C21.Neg(C21)
	C31 := new(big.Int).Sub(new(big.Int).Mul(bVal, f), new(big.Int).Mul(c, e))
	vv1 := new(big.Int).SetUint64(v1)
	vv2 := new(big.Int).SetUint64(v2)
	vv3 := new(big.Int).SetUint64(v3)
	rx := new(big.Int).Mul(C11, vv1)
	rx.Add(rx, new(big.Int).Mul(C21, vv2)).Add(rx, new(big.Int).Mul(C31, vv3))
	rx.Mul(rx, detInv).Mod(rx, p)
	resB := new(big.Int).Sub(rx, s)
	resB.Add(resB, p).Mod(resB, p)
	return byte(resB.Uint64() & 0xFF)
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

func LCTUnpack(hexData string, seed []byte) []byte {
	buf, err := hex.DecodeString(hexData)
	if err != nil || len(buf)%24 != 0 {
		return nil
	}
	st := NewLCTState(seed)
	var output []byte
	for i := 0; i < len(buf); i += 24 {
		v1 := binary.BigEndian.Uint64(buf[i:])
		v2 := binary.BigEndian.Uint64(buf[i+8:])
		v3 := binary.BigEndian.Uint64(buf[i+16:])
		b := st.Unmix(v1, v2, v3)
		output = append(output, b)
		st.Evolve(b)
	}
	return output
}
