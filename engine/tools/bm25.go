package tools

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
)

// chunk 是原文的一块；offset 是它在原文里的 rune 位置。
type chunk struct {
	text   string
	offset int
	tokens int
}

// chunks 把原文按行累积成约 size token 一块；一行超过 size 时按字切开（E.11.02 §4）。
func chunks(text string, size int, cnt Counter) []chunk {
	var out []chunk
	var b strings.Builder
	start, pos := 0, 0
	flush := func() {
		if b.Len() == 0 {
			return
		}
		s := b.String()
		out = append(out, chunk{text: s, offset: start, tokens: cnt.CountText(s)})
		b.Reset()
		start = pos
	}
	for _, line := range strings.SplitAfter(text, "\n") {
		if line == "" {
			continue
		}
		for _, piece := range splitLong(line, size, cnt) {
			if b.Len() > 0 && cnt.CountText(b.String()+piece) > size {
				flush()
			}
			b.WriteString(piece)
			pos += utf8.RuneCountInString(piece)
		}
	}
	flush()
	return out
}

// splitLong 把超过 size token 的一行切成若干段，每段不超过 size。
func splitLong(line string, size int, cnt Counter) []string {
	if cnt.CountText(line) <= size {
		return []string{line}
	}
	var out []string
	rest := line
	for rest != "" {
		p := cut(rest, size, cnt)
		if p == "" {
			_, n := utf8.DecodeRuneInString(rest)
			p = rest[:n]
		}
		out = append(out, p)
		rest = rest[len(p):]
	}
	return out
}

func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r)
}

// terms 切词：ASCII 等字母数字按词（小写），中日韩字按相邻两字。
func terms(s string) []string {
	var out []string
	var word []rune
	prev := rune(-1)
	flush := func() {
		if len(word) > 0 {
			out = append(out, string(word))
			word = word[:0]
		}
	}
	for _, r := range strings.ToLower(s) {
		switch {
		case isCJK(r):
			flush()
			if prev >= 0 {
				out = append(out, string([]rune{prev, r}))
			}
			prev = r
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			prev = -1
			word = append(word, r)
		default:
			flush()
			prev = -1
		}
	}
	flush()
	return out
}

const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

// bm25 给每块打分（k1=1.2，b=0.75）。查询里重复的词只算一次。
func bm25(query string, cs []chunk) []float64 {
	seen := map[string]bool{}
	var q []string
	for _, t := range terms(query) {
		if !seen[t] {
			seen[t] = true
			q = append(q, t)
		}
	}
	n := len(cs)
	scores := make([]float64, n)
	if n == 0 || len(q) == 0 {
		return scores
	}
	tf := make([]map[string]int, n)
	lens := make([]float64, n)
	total := 0.0
	df := map[string]int{}
	for i, c := range cs {
		m := map[string]int{}
		ts := terms(c.text)
		for _, t := range ts {
			m[t]++
		}
		tf[i] = m
		lens[i] = float64(len(ts))
		total += lens[i]
		for _, t := range q {
			if m[t] > 0 {
				df[t]++
			}
		}
	}
	avg := total / float64(n)
	if avg == 0 {
		avg = 1
	}
	for _, t := range q {
		d := float64(df[t])
		idf := math.Log(1 + (float64(n)-d+0.5)/(d+0.5))
		for i := range cs {
			f := float64(tf[i][t])
			if f == 0 {
				continue
			}
			scores[i] += idf * f * (bm25K1 + 1) / (f + bm25K1*(1-bm25B+bm25B*lens[i]/avg))
		}
	}
	return scores
}
