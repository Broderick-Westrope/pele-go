package jsmath

import "math"

// Acos is Math.acos: fdlibm's acos from V8's src/base/ieee754.cc, contracted as clang does
// for Node's arm64 build (see Sin). It shares Asin's constants.
func Acos(x float64) float64 {
	const pi = 3.14159265358979311600e+00
	hx := highWord(x)
	ix := hx & 0x7fffffff
	if ix >= 0x3ff00000 {
		if (ix-0x3ff00000)|int32(lowWord(x)) == 0 {
			if hx > 0 {
				return 0
			}
			return math.FMA(2, pio2Lo, pi)
		}
		return math.NaN()
	}
	if ix < 0x3fe00000 {
		if ix <= 0x3c600000 {
			return pio2Hi + pio2Lo
		}
		z := m(x, x)
		p := m(z, math.FMA(z, math.FMA(z, math.FMA(z, math.FMA(z, math.FMA(z, pS5, pS4), pS3), pS2), pS1), pS0))
		q := math.FMA(z, math.FMA(z, math.FMA(z, math.FMA(z, qS4, qS3), qS2), qS1), 1)
		r := p / q
		return pio2Hi - (x - math.FMA(-x, r, pio2Lo))
	}
	if hx < 0 {
		z := m(1+x, 0.5)
		p := m(z, math.FMA(z, math.FMA(z, math.FMA(z, math.FMA(z, math.FMA(z, pS5, pS4), pS3), pS2), pS1), pS0))
		q := math.FMA(z, math.FMA(z, math.FMA(z, math.FMA(z, qS4, qS3), qS2), qS1), 1)
		s := math.Sqrt(z)
		r := p / q
		w := math.FMA(r, s, -pio2Lo)
		return math.FMA(-2, s+w, pi)
	}
	z := m(1-x, 0.5)
	s := math.Sqrt(z)
	df := math.Float64frombits(math.Float64bits(s) &^ 0xffffffff)
	c := math.FMA(-df, df, z) / (s + df)
	p := m(z, math.FMA(z, math.FMA(z, math.FMA(z, math.FMA(z, math.FMA(z, pS5, pS4), pS3), pS2), pS1), pS0))
	q := math.FMA(z, math.FMA(z, math.FMA(z, math.FMA(z, qS4, qS3), qS2), qS1), 1)
	r := p / q
	w := math.FMA(r, s, c)
	return m(2, df+w)
}
