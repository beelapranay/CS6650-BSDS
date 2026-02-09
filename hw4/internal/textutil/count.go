package textutil

import (
	"regexp"
	"strings"
)

var wordRe = regexp.MustCompile(`[A-Za-z]+(?:'[A-Za-z]+)?`)

func CountWords(text []byte) map[string]int {
	m := make(map[string]int)
	words := wordRe.FindAllString(string(text), -1)
	for _, w := range words {
		w = strings.ToLower(w)
		m[w]++
	}
	return m
}

func MergeCounts(dst map[string]int, src map[string]int) {
	for k, v := range src {
		dst[k] += v
	}
}