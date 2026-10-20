// Package jsjson is JSON.parse: it reads JSON into jsobj values, keeping key order the way
// JS objects enumerate it and keeping lone surrogates from \u escapes.
package jsjson

import (
	"errors"
	"strconv"

	"github.com/Broderick-Westrope/pele-go/internal/jsobj"
	"github.com/Broderick-Westrope/pele-go/internal/jsstr"
)

var errSyntax = errors.New("invalid JSON")

// Parse returns JSON.parse(s), or an error where JSON.parse throws.
func Parse(s string) (any, error) {
	p := parser{src: jsstr.UTF16(s)}
	p.space()
	v, err := p.value()
	if err != nil {
		return nil, err
	}
	p.space()
	if p.i != len(p.src) {
		return nil, errSyntax
	}
	return v, nil
}

type parser struct {
	src []uint16
	i   int
}

func (p *parser) space() {
	for p.i < len(p.src) {
		switch p.src[p.i] {
		case ' ', '\t', '\n', '\r':
			p.i++
		default:
			return
		}
	}
}

func (p *parser) literal(word string) bool {
	if p.i+len(word) > len(p.src) {
		return false
	}
	for k := range len(word) {
		if p.src[p.i+k] != uint16(word[k]) {
			return false
		}
	}
	p.i += len(word)
	return true
}

func (p *parser) value() (any, error) {
	if p.i >= len(p.src) {
		return nil, errSyntax
	}
	switch c := p.src[p.i]; {
	case c == '{':
		return p.object()
	case c == '[':
		return p.array()
	case c == '"':
		return p.string()
	case c == '-' || (c >= '0' && c <= '9'):
		return p.number()
	case p.literal("true"):
		return true, nil
	case p.literal("false"):
		return false, nil
	case p.literal("null"):
		return nil, nil
	}
	return nil, errSyntax
}

func (p *parser) object() (any, error) {
	p.i++ // {
	obj := jsobj.New()
	p.space()
	if p.i < len(p.src) && p.src[p.i] == '}' {
		p.i++
		return obj, nil
	}
	for {
		p.space()
		if p.i >= len(p.src) || p.src[p.i] != '"' {
			return nil, errSyntax
		}
		key, err := p.string()
		if err != nil {
			return nil, err
		}
		p.space()
		if p.i >= len(p.src) || p.src[p.i] != ':' {
			return nil, errSyntax
		}
		p.i++
		p.space()
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		obj.Set(key, v)
		p.space()
		if p.i >= len(p.src) {
			return nil, errSyntax
		}
		switch p.src[p.i] {
		case ',':
			p.i++
		case '}':
			p.i++
			return obj, nil
		default:
			return nil, errSyntax
		}
	}
}

func (p *parser) array() (any, error) {
	p.i++ // [
	out := []any{}
	p.space()
	if p.i < len(p.src) && p.src[p.i] == ']' {
		p.i++
		return out, nil
	}
	for {
		p.space()
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		out = append(out, v)
		p.space()
		if p.i >= len(p.src) {
			return nil, errSyntax
		}
		switch p.src[p.i] {
		case ',':
			p.i++
		case ']':
			p.i++
			return out, nil
		default:
			return nil, errSyntax
		}
	}
}

func (p *parser) string() (string, error) {
	p.i++ // "
	var units []uint16
	for p.i < len(p.src) {
		c := p.src[p.i]
		switch {
		case c == '"':
			p.i++
			return jsstr.FromUTF16(units), nil
		case c < 0x20:
			return "", errSyntax
		case c != '\\':
			units = append(units, c)
			p.i++
			continue
		}
		p.i++
		if p.i >= len(p.src) {
			return "", errSyntax
		}
		e := p.src[p.i]
		p.i++
		switch e {
		case '"', '\\', '/':
			units = append(units, e)
		case 'b':
			units = append(units, '\b')
		case 'f':
			units = append(units, '\f')
		case 'n':
			units = append(units, '\n')
		case 'r':
			units = append(units, '\r')
		case 't':
			units = append(units, '\t')
		case 'u':
			if p.i+4 > len(p.src) {
				return "", errSyntax
			}
			var u uint16
			for k := range 4 {
				d := hexValue(p.src[p.i+k])
				if d < 0 {
					return "", errSyntax
				}
				u = u<<4 | uint16(d)
			}
			p.i += 4
			units = append(units, u)
		default:
			return "", errSyntax
		}
	}
	return "", errSyntax
}

func hexValue(c uint16) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

func (p *parser) number() (any, error) {
	start := p.i
	if p.src[p.i] == '-' {
		p.i++
	}
	digits := func() int {
		n := 0
		for p.i < len(p.src) && p.src[p.i] >= '0' && p.src[p.i] <= '9' {
			p.i++
			n++
		}
		return n
	}
	if p.i < len(p.src) && p.src[p.i] == '0' {
		p.i++
	} else if digits() == 0 {
		return nil, errSyntax
	}
	if p.i < len(p.src) && p.src[p.i] == '.' {
		p.i++
		if digits() == 0 {
			return nil, errSyntax
		}
	}
	if p.i < len(p.src) && (p.src[p.i] == 'e' || p.src[p.i] == 'E') {
		p.i++
		if p.i < len(p.src) && (p.src[p.i] == '+' || p.src[p.i] == '-') {
			p.i++
		}
		if digits() == 0 {
			return nil, errSyntax
		}
	}
	text := jsstr.FromUTF16(p.src[start:p.i])
	v, err := strconv.ParseFloat(text, 64)
	var numErr *strconv.NumError
	if err != nil && !(errors.As(err, &numErr) && numErr.Err == strconv.ErrRange) {
		return nil, errSyntax
	}
	return v, nil
}
