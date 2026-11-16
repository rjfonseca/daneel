//nolint:revive // package comment not required
package search

import (
	"context"
	"encoding/binary"
	"math"
)

// Embedder generates dense vector embeddings for text chunks and search queries.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Model() string
	Dimensions() int
	Close() error
}

// QueryEmbedder is an optional extension for embedders that apply distinct query prefixes.
type QueryEmbedder interface {
	EmbedQuery(ctx context.Context, query string) ([]float32, error)
}

func embedQuery(ctx context.Context, embedder Embedder, query string) ([]float32, error) {
	if qe, ok := embedder.(QueryEmbedder); ok {
		return qe.EmbedQuery(ctx, query)
	}
	vecs, err := embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, nil
	}
	return vecs[0], nil
}

// normalizeL2 normalizes a vector to unit L2 norm in-place (returning a copy if needed)
// so that dot product equals cosine similarity.
func normalizeL2(v []float32) []float32 {
	if len(v) == 0 {
		return v
	}
	var sumSq float64
	for _, x := range v {
		sumSq += float64(x) * float64(x)
	}
	if sumSq == 0 {
		return v
	}
	invNorm := float32(1.0 / math.Sqrt(sumSq))
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = x * invNorm
	}
	return out
}

func dotProduct(a, b []float32) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var sum float64
	for i := 0; i < n; i++ {
		sum += float64(a[i]) * float64(b[i])
	}
	return sum
}

// dotProductBlobLE computes the dot product between a float32 query vector and a
// little-endian float32 BLOB without allocating a temporary float32 slice.
func dotProductBlobLE(query []float32, blob []byte) float64 {
	n := len(query)
	if len(blob) < n*4 {
		n = len(blob) / 4
	}
	var sum0, sum1, sum2, sum3 float64
	i := 0
	for ; i+3 < n; i += 4 {
		off := i * 4
		b0 := math.Float32frombits(binary.LittleEndian.Uint32(blob[off:]))
		b1 := math.Float32frombits(binary.LittleEndian.Uint32(blob[off+4:]))
		b2 := math.Float32frombits(binary.LittleEndian.Uint32(blob[off+8:]))
		b3 := math.Float32frombits(binary.LittleEndian.Uint32(blob[off+12:]))
		sum0 += float64(query[i]) * float64(b0)
		sum1 += float64(query[i+1]) * float64(b1)
		sum2 += float64(query[i+2]) * float64(b2)
		sum3 += float64(query[i+3]) * float64(b3)
	}
	sum := sum0 + sum1 + sum2 + sum3
	for ; i < n; i++ {
		b := math.Float32frombits(binary.LittleEndian.Uint32(blob[i*4:]))
		sum += float64(query[i]) * float64(b)
	}
	return sum
}

func encodeVector(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, x := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(x))
	}
	return buf
}

func decodeVector(b []byte) []float32 {
	if len(b) == 0 || len(b)%4 != 0 {
		return nil
	}
	out := make([]float32, len(b)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return out
}
