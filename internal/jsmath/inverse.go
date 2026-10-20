package jsmath

import "math"

// Asin, Atan and Atan2 are fdlibm's from V8's src/base/ieee754.cc, contracted as clang does
// for Node's arm64 build: see Sin.

const (
	pio2Hi = 1.57079632679489655800e+00
	pio2Lo = 6.12323399573676603587e-17
	pio4Hi = 7.85398163397448278999e-01
	pS0    = 1.66666666666666657415e-01
	pS1    = -3.25565818622400915405e-01
	pS2    = 2.01212532134862925881e-01
	pS3    = -4.00555345006794114027e-02
	pS4    = 7.91534994289814532176e-04
	pS5    = 3.47933107596021167570e-05
	qS1    = -2.40339491173441421878e+00
	qS2    = 2.02094576023350569471e+00
	qS3    = -6.88283971605453293030e-01
	qS4    = 7.70381505559019352791e-02
)

// Asin is Math.asin.
func Asin(x float64) float64 {
	hx := highWord(x)
	ix := hx & 0x7fffffff
	t := 0.0
	if ix >= 0x3ff00000 {
		if (ix-0x3ff00000)|int32(lowWord(x)) == 0 {
			return math.FMA(x, pio2Hi, m(x, pio2Lo))
		}
		return math.NaN()
	} else if ix < 0x3fe00000 {
		if ix < 0x3e400000 {
			if 1e300+x > 1 {
				return x
			}
		} else {
			t = m(x, x)
		}
		p := m(t, math.FMA(t, math.FMA(t, math.FMA(t, math.FMA(t, math.FMA(t, pS5, pS4), pS3), pS2), pS1), pS0))
		q := math.FMA(t, math.FMA(t, math.FMA(t, math.FMA(t, qS4, qS3), qS2), qS1), 1)
		w := p / q
		return math.FMA(x, w, x)
	}
	w := 1 - math.Abs(x)
	t = m(w, 0.5)
	p := m(t, math.FMA(t, math.FMA(t, math.FMA(t, math.FMA(t, math.FMA(t, pS5, pS4), pS3), pS2), pS1), pS0))
	q := math.FMA(t, math.FMA(t, math.FMA(t, math.FMA(t, qS4, qS3), qS2), qS1), 1)
	s := math.Sqrt(t)
	if ix >= 0x3fef3333 {
		w = p / q
		t = pio2Hi - math.FMA(2, math.FMA(s, w, s), -pio2Lo)
	} else {
		w = math.Float64frombits(math.Float64bits(s) &^ 0xffffffff)
		c := math.FMA(-w, w, t) / (s + w)
		r := p / q
		p = math.FMA(m(2, s), r, -math.FMA(-2, c, pio2Lo))
		q = math.FMA(-2, w, pio4Hi)
		t = pio4Hi - (p - q)
	}
	if hx > 0 {
		return t
	}
	return -t
}

var (
	atanHi = [...]float64{4.63647609000806093515e-01, 7.85398163397448278999e-01, 9.82793723247329054082e-01, 1.57079632679489655800e+00}
	atanLo = [...]float64{2.26987774529616870924e-17, 3.06161699786838301793e-17, 1.39033110312309984516e-17, 6.12323399573676603587e-17}
	aT     = [...]float64{
		3.33333333333329318027e-01, -1.99999999998764832476e-01, 1.42857142725034663711e-01,
		-1.11111104054623557880e-01, 9.09088713343650656196e-02, -7.69187620504482999495e-02,
		6.66107313738753120669e-02, -5.83357013379057348645e-02, 4.97687799461593236017e-02,
		-3.65315727442169155270e-02, 1.62858201153657823623e-02,
	}
)

// Atan is Math.atan.
func Atan(x float64) float64 {
	hx := highWord(x)
	ix := hx & 0x7fffffff
	if ix >= 0x44100000 {
		if ix > 0x7ff00000 || (ix == 0x7ff00000 && lowWord(x) != 0) {
			return x + x
		}
		if hx > 0 {
			return atanHi[3] + atanLo[3]
		}
		return -atanHi[3] - atanLo[3]
	}
	id := -1
	if ix < 0x3fdc0000 {
		if ix < 0x3e400000 && 1e300+x > 1 {
			return x
		}
	} else {
		x = math.Abs(x)
		switch {
		case ix < 0x3fe60000:
			id = 0
			x = math.FMA(2, x, -1) / (2 + x)
		case ix < 0x3ff30000:
			id = 1
			x = (x - 1) / (x + 1)
		case ix < 0x40038000:
			id = 2
			x = (x - 1.5) / math.FMA(1.5, x, 1)
		default:
			id = 3
			x = -1 / x
		}
	}
	z := m(x, x)
	w := m(z, z)
	s1 := m(z, math.FMA(w, math.FMA(w, math.FMA(w, math.FMA(w, math.FMA(w, aT[10], aT[8]), aT[6]), aT[4]), aT[2]), aT[0]))
	s2 := m(w, math.FMA(w, math.FMA(w, math.FMA(w, math.FMA(w, aT[9], aT[7]), aT[5]), aT[3]), aT[1]))
	if id < 0 {
		return math.FMA(-x, s1+s2, x)
	}
	z = atanHi[id] - (math.FMA(x, s1+s2, -atanLo[id]) - x)
	if hx < 0 {
		return -z
	}
	return z
}

// Atan2 is Math.atan2.
func Atan2(y, x float64) float64 {
	const (
		tiny = 1.0e-300
		pio4 = 7.8539816339744827900e-01
		pio2 = 1.5707963267948965580e+00
		pi   = 3.1415926535897931160e+00
		piLo = 1.2246467991473531772e-16
	)
	hx, lx := highWord(x), lowWord(x)
	ix := hx & 0x7fffffff
	hy, ly := highWord(y), lowWord(y)
	iy := hy & 0x7fffffff
	if ix|int32((lx|-lx)>>31) > 0x7ff00000 || iy|int32((ly|-ly)>>31) > 0x7ff00000 {
		return x + y
	}
	if (hx-0x3ff00000)|int32(lx) == 0 {
		return Atan(y)
	}
	k := ((hy >> 31) & 1) | ((hx >> 30) & 2)
	if iy|int32(ly) == 0 {
		switch k {
		case 0, 1:
			return y
		case 2:
			return pi + tiny
		default:
			return -pi - tiny
		}
	}
	if ix|int32(lx) == 0 {
		if hy < 0 {
			return -pio2 - tiny
		}
		return pio2 + tiny
	}
	if ix == 0x7ff00000 {
		if iy == 0x7ff00000 {
			switch k {
			case 0:
				return pio4 + tiny
			case 1:
				return -pio4 - tiny
			case 2:
				return math.FMA(3, pio4, tiny)
			default:
				return math.FMA(-3, pio4, -tiny)
			}
		}
		switch k {
		case 0:
			return 0
		case 1:
			return math.Copysign(0, -1)
		case 2:
			return pi + tiny
		default:
			return -pi - tiny
		}
	}
	if iy == 0x7ff00000 {
		if hy < 0 {
			return -pio2 - tiny
		}
		return pio2 + tiny
	}
	var z float64
	switch e := (iy - ix) >> 20; {
	case e > 60:
		z = math.FMA(0.5, piLo, pio2)
		k &= 1
	case hx < 0 && e < -60:
		z = 0
	default:
		z = Atan(math.Abs(y / x))
	}
	switch k {
	case 0:
		return z
	case 1:
		return -z
	case 2:
		return pi - (z - piLo)
	default:
		return (z - piLo) - pi
	}
}
