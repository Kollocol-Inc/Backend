package lang

import (
	"slices"
	"strings"
)

type Lang string

const (
	RU      Lang = "ru"
	EN      Lang = "en"
	Default      = RU
)

var Supported = []Lang{RU, EN}

func IsSupported(l Lang) bool {
	return slices.Contains(Supported, l)
}

func Parse(s string) Lang {
	l := Lang(strings.ToLower(strings.TrimSpace(s)))
	if IsSupported(l) {
		return l
	}
	return Default
}
