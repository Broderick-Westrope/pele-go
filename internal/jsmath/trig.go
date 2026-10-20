package jsmath

import "math"

// Sin and Cos are fdlibm's, as V8 computes Math.sin and Math.cos (src/base/ieee754.cc), and
// as Node's arm64 build compiles them: clang contracts a*b+c within an expression into one
// fused multiply-add (-ffp-contract=on), so those are math.FMA here and every other product
// is rounded on its own (see m). Go's math.Sin and math.Cos differ in the last bit for many
// arguments. A V8 built without contraction (x86-64) differs from both in a few.

func highWord(x float64) int32 { return int32(math.Float64bits(x) >> 32) }
func lowWord(x float64) uint32 { return uint32(math.Float64bits(x)) }
func fromWords(hi int32, lo uint32) float64 {
	return math.Float64frombits(uint64(uint32(hi))<<32 | uint64(lo))
}

// Sin is Math.sin.
func Sin(x float64) float64 {
	ix := highWord(x) & 0x7fffffff
	switch {
	case ix <= 0x3fe921fb:
		return kernelSin(x, 0, false)
	case ix >= 0x7ff00000:
		return x - x
	}
	n, y0, y1 := remPio2(x)
	switch n & 3 {
	case 0:
		return kernelSin(y0, y1, true)
	case 1:
		return kernelCos(y0, y1)
	case 2:
		return -kernelSin(y0, y1, true)
	default:
		return -kernelCos(y0, y1)
	}
}

// Cos is Math.cos.
func Cos(x float64) float64 {
	ix := highWord(x) & 0x7fffffff
	switch {
	case ix <= 0x3fe921fb:
		return kernelCos(x, 0)
	case ix >= 0x7ff00000:
		return x - x
	}
	n, y0, y1 := remPio2(x)
	switch n & 3 {
	case 0:
		return kernelCos(y0, y1)
	case 1:
		return -kernelSin(y0, y1, true)
	case 2:
		return -kernelCos(y0, y1)
	default:
		return kernelSin(y0, y1, true)
	}
}

// The kernels' polynomial coefficients.
const (
	S1 = -1.66666666666666324348e-01
	S2 = 8.33333333332248946124e-03
	S3 = -1.98412698298579493134e-04
	S4 = 2.75573137070700676789e-06
	S5 = -2.50507602534068634195e-08
	S6 = 1.58969099521155010221e-10
	C1 = 4.16666666666666019037e-02
	C2 = -1.38888888888741095749e-03
	C3 = 2.48015872894767294178e-05
	C4 = -2.75573143513906633035e-07
	C5 = 2.08757232129817482790e-09
	C6 = -1.13596475577881948265e-11
)

// m multiplies, rounding the product so the compiler cannot fuse it into an add.
func m(a, b float64) float64 { return float64(a * b) }

// kernelSin is sin on [-pi/4, pi/4]; y is the tail of x, used when tail is set.
func kernelSin(x, y float64, tail bool) float64 {
	ix := highWord(x) & 0x7fffffff
	if ix < 0x3e400000 && int32(x) == 0 {
		return x
	}
	z := m(x, x)
	v := m(z, x)
	r := math.FMA(z, math.FMA(z, math.FMA(z, math.FMA(z, S6, S5), S4), S3), S2)
	if !tail {
		return math.FMA(v, math.FMA(z, r, S1), x)
	}
	return x - math.FMA(-v, S1, math.FMA(z, math.FMA(0.5, y, -m(v, r)), -y))
}

// kernelCos is cos on [-pi/4, pi/4]; y is the tail of x.
func kernelCos(x, y float64) float64 {
	ix := highWord(x) & 0x7fffffff
	if ix < 0x3e400000 && int32(x) == 0 {
		return 1
	}
	z := m(x, x)
	r := m(z, math.FMA(z, math.FMA(z, math.FMA(z, math.FMA(z, math.FMA(z, C6, C5), C4), C3), C2), C1))
	if ix < 0x3fd33333 {
		return 1 - math.FMA(0.5, z, -math.FMA(z, r, -m(x, y)))
	}
	var qx float64
	if ix > 0x3fe90000 {
		qx = 0.28125
	} else {
		qx = fromWords(ix-0x00200000, 0)
	}
	hz := math.FMA(0.5, z, -qx)
	a := 1 - qx
	return a - (hz - math.FMA(z, r, -m(x, y)))
}

// npio2HW holds the high words of n*pi/2 for n from 1 to 32.
var npio2HW = [...]int32{
	0x3FF921FB, 0x400921FB, 0x4012D97C, 0x401921FB, 0x401F6A7A, 0x4022D97C, 0x4025FDBB, 0x402921FB,
	0x402C463A, 0x402F6A7A, 0x4031475C, 0x4032D97C, 0x40346B9C, 0x4035FDBB, 0x40378FDB, 0x403921FB,
	0x403AB41B, 0x403C463A, 0x403DD85A, 0x403F6A7A, 0x40407E4C, 0x4041475C, 0x4042106C, 0x4042D97C,
	0x4043A28C, 0x40446B9C, 0x404534AC, 0x4045FDBB, 0x4046C6CB, 0x40478FDB, 0x404858EB, 0x404921FB,
}

// pi/2 in pieces, and 2/pi.
const (
	two24   = 1.67772160000000000000e+07
	twon24  = 5.96046447753906250000e-08
	invpio2 = 6.36619772367581382433e-01
	pio2_1  = 1.57079632673412561417e+00
	pio2_1t = 6.07710050650619224932e-11
	pio2_2  = 6.07710050630396597660e-11
	pio2_2t = 2.02226624879595063154e-21
	pio2_3  = 2.02226624871116645580e-21
	pio2_3t = 8.47842766036889956997e-32
)

// remPio2 returns x reduced by n*pi/2, as y0 + y1, and n.
func remPio2(x float64) (n int32, y0, y1 float64) {
	hx := highWord(x)
	ix := hx & 0x7fffffff
	if ix < 0x4002d97c {
		if hx > 0 {
			z := x - pio2_1
			if ix != 0x3ff921fb {
				y0 = z - pio2_1t
				y1 = (z - y0) - pio2_1t
			} else {
				z -= pio2_2
				y0 = z - pio2_2t
				y1 = (z - y0) - pio2_2t
			}
			return 1, y0, y1
		}
		z := x + pio2_1
		if ix != 0x3ff921fb {
			y0 = z + pio2_1t
			y1 = (z - y0) + pio2_1t
		} else {
			z += pio2_2
			y0 = z + pio2_2t
			y1 = (z - y0) + pio2_2t
		}
		return -1, y0, y1
	}
	if ix <= 0x413921fb {
		t := math.Abs(x)
		n = int32(math.FMA(t, invpio2, 0.5))
		fn := float64(n)
		r := math.FMA(-fn, pio2_1, t)
		w := m(fn, pio2_1t)
		if n < 32 && ix != npio2HW[n-1] {
			y0 = r - w
		} else {
			j := ix >> 20
			y0 = r - w
			i := j - ((highWord(y0) >> 20) & 0x7ff)
			if i > 16 {
				t = r
				w = m(fn, pio2_2)
				r = t - w
				w = math.FMA(fn, pio2_2t, -((t - r) - w))
				y0 = r - w
				i = j - ((highWord(y0) >> 20) & 0x7ff)
				if i > 49 {
					t = r
					w = m(fn, pio2_3)
					r = t - w
					w = math.FMA(fn, pio2_3t, -((t - r) - w))
					y0 = r - w
				}
			}
		}
		y1 = (r - y0) - w
		if hx < 0 {
			return -n, -y0, -y1
		}
		return n, y0, y1
	}
	if ix >= 0x7ff00000 {
		return 0, x - x, x - x
	}
	e0 := (ix >> 20) - 1046
	z := fromWords(ix-e0<<20, lowWord(x))
	var tx [3]float64
	for i := range 2 {
		tx[i] = float64(int32(z))
		z = m(z-tx[i], two24)
	}
	tx[2] = z
	nx := 3
	for tx[nx-1] == 0 {
		nx--
	}
	n, y0, y1 = kernelRemPio2(tx[:nx], int(e0))
	if hx < 0 {
		return -n, -y0, -y1
	}
	return n, y0, y1
}

// twoOverPi is 2/pi in 24-bit chunks.
var twoOverPi = [...]int32{
	0xA2F983, 0x6E4E44, 0x1529FC, 0x2757D1, 0xF534DD, 0xC0DB62, 0x95993C, 0x439041, 0xFE5163,
	0xABDEBB, 0xC561B7, 0x246E3A, 0x424DD2, 0xE00649, 0x2EEA09, 0xD1921C, 0xFE1DEB, 0x1CB129,
	0xA73EE8, 0x8235F5, 0x2EBB44, 0x84E99C, 0x7026B4, 0x5F7E41, 0x3991D6, 0x398353, 0x39F49C,
	0x845F8B, 0xBDF928, 0x3B1FF8, 0x97FFDE, 0x05980F, 0xEF2F11, 0x8B5A0A, 0x6D1F6D, 0x367ECF,
	0x27CB09, 0xB74F46, 0x3F669E, 0x5FEA2D, 0x7527BA, 0xC7EBE5, 0xF17B3D, 0x0739F7, 0x8A5292,
	0xEA6BFB, 0x5FB11F, 0x8D5D08, 0x560330, 0x46FC7B, 0x6BABF0, 0xCFBC20, 0x9AF436, 0x1DA9E3,
	0x91615E, 0xE61B08, 0x659985, 0x5F14A0, 0x68408D, 0xFFD880, 0x4D7327, 0x310606, 0x1556CA,
	0x73A8C9, 0x60E27B, 0xC08C6B,
}

var pio2Parts = [...]float64{
	1.57079625129699707031e+00,
	7.54978941586159635335e-08,
	5.39030252995776476554e-15,
	3.28200341580791294123e-22,
	1.27065575308067607349e-29,
	1.22933308981111328932e-36,
	2.73370053816464559624e-44,
	2.16741683877804819444e-51,
}

// kernelRemPio2 is fdlibm's __kernel_rem_pio2 at the precision V8 asks for (prec 2).
func kernelRemPio2(x []float64, e0 int) (int32, float64, float64) {
	const jk = 4
	const jp = jk
	var iq [20]int32
	var f, fq, q [20]float64
	jx := len(x) - 1
	jv := max((e0-3)/24, 0)
	q0 := e0 - 24*(jv+1)

	for i, j := 0, jv-jx; i <= jx+jk; i, j = i+1, j+1 {
		if j >= 0 {
			f[i] = float64(twoOverPi[j])
		}
	}
	for i := 0; i <= jk; i++ {
		fw := 0.0
		for j := 0; j <= jx; j++ {
			fw = math.FMA(x[j], f[jx+i-j], fw)
		}
		q[i] = fw
	}

	jz := jk
	var z float64
	var n, ih int32
	for {
		i := 0
		z = q[jz]
		for j := jz; j > 0; i, j = i+1, j-1 {
			fw := float64(int32(m(twon24, z)))
			iq[i] = int32(math.FMA(-two24, fw, z))
			z = q[j-1] + fw
		}

		z = math.Ldexp(z, q0)
		z = math.FMA(-8.0, math.Floor(m(z, 0.125)), z)
		n = int32(z)
		z -= float64(n)
		ih = 0
		switch {
		case q0 > 0:
			i := iq[jz-1] >> (24 - q0)
			n += i
			iq[jz-1] -= i << (24 - q0)
			ih = iq[jz-1] >> (23 - q0)
		case q0 == 0:
			ih = iq[jz-1] >> 23
		case z >= 0.5:
			ih = 2
		}

		if ih > 0 {
			n++
			carry := 0
			for i := range jz {
				j := iq[i]
				if carry == 0 {
					if j != 0 {
						carry = 1
						iq[i] = 0x1000000 - j
					}
				} else {
					iq[i] = 0xffffff - j
				}
			}
			switch q0 {
			case 1:
				iq[jz-1] &= 0x7fffff
			case 2:
				iq[jz-1] &= 0x3fffff
			}
			if ih == 2 {
				z = 1 - z
				if carry != 0 {
					z -= math.Ldexp(1, q0)
				}
			}
		}

		if z != 0 {
			break
		}
		j := int32(0)
		for i := jz - 1; i >= jk; i-- {
			j |= iq[i]
		}
		if j != 0 {
			break
		}
		k := 1
		for iq[jk-k] == 0 {
			k++
		}
		for i := jz + 1; i <= jz+k; i++ {
			f[jx+i] = float64(twoOverPi[jv+i])
			fw := 0.0
			for j := 0; j <= jx; j++ {
				fw = math.FMA(x[j], f[jx+i-j], fw)
			}
			q[i] = fw
		}
		jz += k
	}

	if z == 0 {
		jz--
		q0 -= 24
		for iq[jz] == 0 {
			jz--
			q0 -= 24
		}
	} else {
		z = math.Ldexp(z, -q0)
		if z >= two24 {
			fw := float64(int32(m(twon24, z)))
			iq[jz] = int32(math.FMA(-two24, fw, z))
			jz++
			q0 += 24
			iq[jz] = int32(fw)
		} else {
			iq[jz] = int32(z)
		}
	}

	fw := math.Ldexp(1, q0)
	for i := jz; i >= 0; i-- {
		q[i] = m(fw, float64(iq[i]))
		fw = m(fw, twon24)
	}
	for i := jz; i >= 0; i-- {
		fw := 0.0
		for k := 0; k <= jp && k <= jz-i; k++ {
			fw = math.FMA(pio2Parts[k], q[i+k], fw)
		}
		fq[jz-i] = fw
	}

	fw = 0
	for i := jz; i >= 0; i-- {
		fw += fq[i]
	}
	y0 := fw
	if ih != 0 {
		y0 = -fw
	}
	fw = fq[0] - fw
	for i := 1; i <= jz; i++ {
		fw += fq[i]
	}
	y1 := fw
	if ih != 0 {
		y1 = -fw
	}
	return n & 7, y0, y1
}
