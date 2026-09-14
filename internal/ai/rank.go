package ai

import (
	"math"
	"sort"
	"strings"
)

// RankedChunk is a chunk with score.
type RankedChunk struct {
	Chunk Chunk
	Score float64
}

// RankTFIDF ranks chunks with TF-IDF cosine over query.
func RankTFIDF(chunks []Chunk, query string, topK int) []RankedChunk {
	toks := tokenize(query)
	if len(toks) == 0 || len(chunks) == 0 {
		return nil
	}
	// document frequency
	df := map[string]int{}
	docs := make([][]string, len(chunks))
	for i, c := range chunks {
		ts := tokenize(c.Text)
		docs[i] = ts
		seen := map[string]bool{}
		for _, t := range ts {
			if !seen[t] {
				seen[t] = true
				df[t]++
			}
		}
	}
	n := float64(len(chunks))
	qTF := termFreq(toks)
	var scored []RankedChunk
	for i, c := range chunks {
		dTF := termFreq(docs[i])
		var dot, qNorm, dNorm float64
		// iterate union of query terms (query-weighted cosine is enough for ranking)
		for term, qf := range qTF {
			idf := math.Log(1 + n/(1+float64(df[term])))
			qw := qf * idf
			dw := dTF[term] * idf
			dot += qw * dw
			qNorm += qw * qw
		}
		for _, v := range dTF {
			_ = v
		}
		// doc norm over its own terms
		for term, tf := range dTF {
			idf := math.Log(1 + n/(1+float64(df[term])))
			w := tf * idf
			dNorm += w * w
		}
		score := 0.0
		if qNorm > 0 && dNorm > 0 {
			score = dot / (math.Sqrt(qNorm) * math.Sqrt(dNorm))
		}
		// small boost for kind matches
		q := strings.ToLower(query)
		kind := strings.ToLower(c.Kind)
		if strings.Contains(q, kind) {
			score += 0.05
		}
		scored = append(scored, RankedChunk{Chunk: c, Score: score})
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].Score > scored[j].Score })
	if topK > 0 && len(scored) > topK {
		scored = scored[:topK]
	}
	return scored
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	return strings.Fields(b.String())
}

func termFreq(toks []string) map[string]float64 {
	tf := map[string]float64{}
	for _, t := range toks {
		tf[t]++
	}
	n := float64(len(toks))
	if n == 0 {
		return tf
	}
	for k := range tf {
		tf[k] /= n
	}
	return tf
}
