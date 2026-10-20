package jsmath

import "math"

// Log10 is Math.log10: fdlibm's, as V8 computes it (src/base/ieee754.cc) and Node's arm64
// build compiles it, with the same contractions as Sin. Go's math.Log10 is log(x)/ln(10),
// which misses exact powers of ten (math.Log10(1000) is 2.9999999999999996).
func Log10(x float64) float64 {
	const (
		two54     = 1.80143985094819840000e+16
		ivln10    = 4.34294481903251816668e-01
		log10_2hi = 3.01029995663611771306e-01
		log10_2lo = 3.69423907715893078616e-13
	)
	hx, lx := highWord(x), lowWord(x)
	k := int32(0)
	if hx < 0x00100000 {
		if (hx&0x7fffffff)|int32(lx) == 0 {
			return math.Inf(-1)
		}
		if hx < 0 {
			return math.NaN()
		}
		k -= 54
		x *= two54
		hx, lx = highWord(x), lowWord(x)
	}
	if hx >= 0x7ff00000 {
		return x + x
	}
	if hx == 0x3ff00000 && lx == 0 {
		return 0
	}
	k += hx>>20 - 1023
	i := int32(uint32(k) & 0x80000000 >> 31)
	hx = hx&0x000fffff | (0x3ff-i)<<20
	y := float64(k + i)
	x = fromWords(hx, lx)
	z := math.FMA(y, log10_2lo, m(ivln10, ieeeLog(x)))
	return math.FMA(y, log10_2hi, z)
}

// ieeeLog is fdlibm's log, which Log10 builds on.
func ieeeLog(x float64) float64 {
	const (
		ln2Hi = 6.93147180369123816490e-01
		ln2Lo = 1.90821492927058770002e-10
		two54 = 1.80143985094819840000e+16
		lg1   = 6.666666666666735130e-01
		lg2   = 3.999999999940941908e-01
		lg3   = 2.857142874366239149e-01
		lg4   = 2.222219843214978396e-01
		lg5   = 1.818357216161805012e-01
		lg6   = 1.531383769920937332e-01
		lg7   = 1.479819860511658591e-01
	)
	hx := highWord(x)
	lx := lowWord(x)
	k := int32(0)
	if hx < 0x00100000 {
		if (hx&0x7fffffff)|int32(lx) == 0 {
			return math.Inf(-1)
		}
		if hx < 0 {
			return math.NaN()
		}
		k -= 54
		x *= two54
		hx = highWord(x)
	}
	if hx >= 0x7ff00000 {
		return x + x
	}
	k += hx>>20 - 1023
	hx &= 0x000fffff
	i := (hx + 0x95f64) & 0x100000
	x = fromWords(hx|(i^0x3ff00000), lowWord(x))
	k += i >> 20
	f := x - 1.0
	if 0x000fffff&(2+hx) < 3 {
		if f == 0 {
			if k == 0 {
				return 0
			}
			dk := float64(k)
			return math.FMA(dk, ln2Hi, m(dk, ln2Lo))
		}
		r := m(m(f, f), math.FMA(-0.33333333333333333, f, 0.5))
		if k == 0 {
			return f - r
		}
		dk := float64(k)
		return math.FMA(dk, ln2Hi, -(math.FMA(-dk, ln2Lo, r) - f))
	}
	s := f / (2.0 + f)
	dk := float64(k)
	z := m(s, s)
	i = hx - 0x6147a
	w := m(z, z)
	j := 0x6b851 - hx
	t1 := m(w, math.FMA(w, math.FMA(w, lg6, lg4), lg2))
	t2 := m(z, math.FMA(w, math.FMA(w, math.FMA(w, lg7, lg5), lg3), lg1))
	i |= j
	r := t2 + t1
	if i > 0 {
		hfsq := m(m(0.5, f), f)
		if k == 0 {
			return f - math.FMA(-s, hfsq+r, hfsq)
		}
		return math.FMA(dk, ln2Hi, -((hfsq - math.FMA(s, hfsq+r, m(dk, ln2Lo))) - f))
	}
	if k == 0 {
		return math.FMA(-s, f-r, f)
	}
	return math.FMA(dk, ln2Hi, -(math.FMA(s, f-r, -m(dk, ln2Lo)) - f))
}
