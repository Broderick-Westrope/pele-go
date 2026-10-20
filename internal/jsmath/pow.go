package jsmath

import "math"

// Pow is Math.pow and the ** operator: fdlibm's pow, as V8 computes it (src/base/ieee754.cc)
// and Node's arm64 build compiles it, with the same contractions as Sin. Go's math.Pow is
// another algorithm and differs in the last bit, also for powers of ten (10 ** -5).
func Pow(x, y float64) float64 {
	const (
		two53   = 9007199254740992.0
		l1      = 5.99999999999994648725e-01
		l2      = 4.28571428578550184252e-01
		l3      = 3.33333329818377432918e-01
		l4      = 2.72728123808534006489e-01
		l5      = 2.30660745775561754067e-01
		l6      = 2.06975017800338417784e-01
		p1      = 1.66666666666666019037e-01
		p2      = -2.77777777770155933842e-03
		p3      = 6.61375632143793436117e-05
		p4      = -1.65339022054652515390e-06
		p5      = 4.13813679705723846039e-08
		lg2     = 6.93147180559945286227e-01
		lg2H    = 6.93147182464599609375e-01
		lg2L    = -1.90465429995776804525e-09
		ovt     = 8.0085662595372944372e-17
		cp      = 9.61796693925975554329e-01
		cpH     = 9.61796700954437255859e-01
		cpL     = -7.02846165095275826516e-09
		ivln2   = 1.44269504088896338700e+00
		ivln2H  = 1.44269502162933349609e+00
		ivln2L  = 1.92596299112661746887e-08
		oneHalf = 0.5
	)
	// Variables, so that huge * huge overflows when it runs, as in C.
	huge, tiny := 1.0e300, 1.0e-300
	bp := [2]float64{1.0, 1.5}
	dpH := [2]float64{0.0, 5.84962487220764160156e-01}
	dpL := [2]float64{0.0, 1.35003920212974897128e-08}

	hx, lx := highWord(x), lowWord(x)
	hy, ly := highWord(y), lowWord(y)
	ix := hx & 0x7fffffff
	iy := hy & 0x7fffffff

	if iy|int32(ly) == 0 {
		return 1
	}
	if ix > 0x7ff00000 || (ix == 0x7ff00000 && lx != 0) || iy > 0x7ff00000 || (iy == 0x7ff00000 && ly != 0) {
		return x + y
	}

	// 0: y is not an integer, 1: an odd one, 2: an even one.
	yisint := int32(0)
	if hx < 0 {
		if iy >= 0x43400000 {
			yisint = 2
		} else if iy >= 0x3ff00000 {
			k := iy>>20 - 0x3ff
			if k > 20 {
				j := int32(ly >> uint(52-k))
				if j<<uint(52-k) == int32(ly) {
					yisint = 2 - j&1
				}
			} else if ly == 0 {
				j := iy >> uint(20-k)
				if j<<uint(20-k) == iy {
					yisint = 2 - j&1
				}
			}
		}
	}

	if ly == 0 {
		if iy == 0x7ff00000 {
			switch {
			case (ix-0x3ff00000)|int32(lx) == 0:
				return y - y
			case ix >= 0x3ff00000:
				if hy >= 0 {
					return y
				}
				return 0
			default:
				if hy < 0 {
					return -y
				}
				return 0
			}
		}
		if iy == 0x3ff00000 {
			if hy < 0 {
				return 1 / x
			}
			return x
		}
		if hy == 0x40000000 {
			return x * x
		}
		if hy == 0x3fe00000 && hx >= 0 {
			return math.Sqrt(x)
		}
	}

	ax := math.Abs(x)
	if lx == 0 && (ix == 0x7ff00000 || ix == 0 || ix == 0x3ff00000) {
		z := ax
		if hy < 0 {
			z = 1 / z
		}
		if hx < 0 {
			if (ix-0x3ff00000)|yisint == 0 {
				z = math.NaN()
			} else if yisint == 1 {
				z = -z
			}
		}
		return z
	}

	n := hx>>31 + 1
	if n|yisint == 0 {
		return math.NaN()
	}
	s := 1.0
	if n|(yisint-1) == 0 {
		s = -1
	}

	var t1, t2 float64
	if iy > 0x41e00000 {
		if iy > 0x43f00000 {
			if ix <= 0x3fefffff {
				if hy < 0 {
					return huge * huge
				}
				return tiny * tiny
			}
			if ix >= 0x3ff00000 {
				if hy > 0 {
					return huge * huge
				}
				return tiny * tiny
			}
		}
		if ix < 0x3fefffff {
			if hy < 0 {
				return s * huge * huge
			}
			return s * tiny * tiny
		}
		if ix > 0x3ff00000 {
			if hy > 0 {
				return s * huge * huge
			}
			return s * tiny * tiny
		}
		t := ax - 1
		w := m(t, t) * math.FMA(-t, math.FMA(-t, 0.25, 0.3333333333333333333333), oneHalf)
		u := m(ivln2H, t)
		v := math.FMA(t, ivln2L, -m(w, ivln2))
		t1 = fromWords(highWord(u+v), 0)
		t2 = v - (t1 - u)
	} else {
		n = 0
		if ix < 0x00100000 {
			ax *= two53
			n -= 53
			ix = highWord(ax)
		}
		n += ix>>20 - 0x3ff
		j := ix & 0x000fffff
		ix = j | 0x3ff00000
		k := 0
		switch {
		case j <= 0x3988e:
		case j < 0xbb67a:
			k = 1
		default:
			n++
			ix -= 0x00100000
		}
		ax = fromWords(ix, lowWord(ax))

		u := ax - bp[k]
		v := 1 / (ax + bp[k])
		ss := u * v
		sH := fromWords(highWord(ss), 0)
		tH := fromWords((ix>>1|0x20000000)+0x00080000+int32(k)<<18, 0)
		tL := ax - (tH - bp[k])
		sL := v * math.FMA(-sH, tL, math.FMA(-sH, tH, u))
		s2 := m(ss, ss)
		r := m(s2, s2) * math.FMA(s2, math.FMA(s2, math.FMA(s2, math.FMA(s2, math.FMA(s2, l6, l5), l4), l3), l2), l1)
		r = math.FMA(sL, sH+ss, r)
		s2 = m(sH, sH)
		tH = fromWords(highWord(3.0+s2+r), 0)
		tL = r - ((tH - 3.0) - s2)
		u = m(sH, tH)
		v = math.FMA(sL, tH, m(tL, ss))
		pH := fromWords(highWord(u+v), 0)
		pL := v - (pH - u)
		zH := m(cpH, pH)
		zL := math.FMA(cpL, pH, m(pL, cp)) + dpL[k]
		t := float64(n)
		t1 = fromWords(highWord(((zH+zL)+dpH[k])+t), 0)
		t2 = zL - (((t1 - t) - dpH[k]) - zH)
	}

	y1 := fromWords(hy, 0)
	pL := math.FMA(y-y1, t1, m(y, t2))
	pH := m(y1, t1)
	z := pL + pH
	j, i := highWord(z), int32(lowWord(z))
	if j >= 0x40900000 {
		if (j-0x40900000)|i != 0 || pL+ovt > z-pH {
			return s * huge * huge
		}
	} else if j&0x7fffffff >= 0x4090cc00 {
		if uint32(j)-0xc090cc00|uint32(i) != 0 || pL <= z-pH {
			return s * tiny * tiny
		}
	}

	i = j & 0x7fffffff
	k := i>>20 - 0x3ff
	n = 0
	if i > 0x3fe00000 {
		n = j + 0x00100000>>uint(k+1)
		k = (n&0x7fffffff)>>20 - 0x3ff
		t := fromWords(n&^(0x000fffff>>uint(k)), 0)
		n = (n&0x000fffff | 0x00100000) >> uint(20-k)
		if j < 0 {
			n = -n
		}
		pH -= t
	}
	t := fromWords(highWord(pL+pH), 0)
	u := m(t, lg2H)
	v := math.FMA(pL-(t-pH), lg2, m(t, lg2L))
	z = u + v
	w := v - (z - u)
	t = m(z, z)
	t1 = math.FMA(-t, math.FMA(t, math.FMA(t, math.FMA(t, math.FMA(t, p5, p4), p3), p2), p1), z)
	r := m(z, t1) / ((t1 - 2) - math.FMA(z, w, w))
	z = 1 - (r - z)
	j = highWord(z)
	j += int32(uint32(n) << 20)
	if j>>20 <= 0 {
		z = math.Ldexp(z, int(n))
	} else {
		z = fromWords(j, lowWord(z))
	}
	return s * z
}
