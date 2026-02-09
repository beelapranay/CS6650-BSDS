package textutil

import (
	"bytes"
)

func SplitIntoN(text []byte, n int) [][]byte {
	if n <= 1 {
		return [][]byte{text}
	}

	lines := bytes.Split(text, []byte("\n"))
	total := len(lines)
	if total == 0 {
		return make([][]byte, n)
	}

	out := make([][]byte, 0, n)
	start := 0
	for i := 0; i < n; i++ {
		// roughly equal number of lines per chunk
		end := (total * (i + 1)) / n
		if end < start {
			end = start
		}
		chunkLines := lines[start:end]
		out = append(out, bytes.Join(chunkLines, []byte("\n")))
		start = end
	}
	return out
}
