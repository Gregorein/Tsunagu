package chapternum

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

var regexes = []*regexp.Regexp{

	regexp.MustCompile(`(?i)(?:chapter|episode)\W*([0-9]+(?:\.[0-9]+)?)`),

	regexp.MustCompile(`(?i)\b(?:chap|ch|ep|#)\W*([0-9]+(?:\.[0-9]+)?)`),

	regexp.MustCompile(`^\s*([0-9]+(?:\.[0-9]+)?)\b`),
}

func FromTitle(title string) float64 {
	t := strings.TrimSpace(title)
	for _, re := range regexes {
		if m := re.FindStringSubmatch(t); m != nil {
			if f, err := strconv.ParseFloat(m[1], 64); err == nil && f > 0 {
				return Round(f)
			}
		}
	}
	return 0
}

// Round constrains a chapter/episode number to 2 decimal places. Extension
// data crosses a Kotlin Float -> Double widening on its way in (see the
// sandbox's gRPC layer), which turns clean values like 19.1 into noise like
// 19.100000381469727; this is the single point every number passes through
// before being persisted, so rounding here fixes it everywhere downstream.
func Round(num float64) float64 {
	return math.Round(num*100) / 100
}

func Resolve(num float64, title string) float64 {
	if num > 0 {
		return Round(num)
	}
	return FromTitle(title)
}
