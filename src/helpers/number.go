package helpers

import (
	"math"
	"strconv"
	"strings"
)

func ParseFloat(s string) (float64, error) {
	s = Clean(s)
	s = strings.ReplaceAll(s, ",", ".")
	s = strings.ReplaceAll(s, " ", "")

	return strconv.ParseFloat(s, 64)
}

func Round2(f float64) float64 {
	return math.Round(f * 100) / 100
}
