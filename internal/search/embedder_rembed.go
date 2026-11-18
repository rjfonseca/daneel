//nolint:revive // package comment not required
package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/rostamlabs/rembed"

	"github.com/rjfonseca/daneel/internal/config"
)

// RembedEmbedder wraps github.com/rostamlabs/rembed for pure-Go BERT embedding inference.
type RembedEmbedder struct {
	inner       *rembed.Embedder
	modelName   string
	queryPrefix string
	docPrefix   string
}

// NewRembedEmbedder loads a pure-Go embedding model via rembed.
func NewRembedEmbedder(cfg config.EmbeddingsConfig) (*RembedEmbedder, error) {
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = config.DefaultEmbeddingsConfig().Model
	}

	var opts []rembed.Option
	if cfg.Int8 {
		opts = append(opts, rembed.WithInt8())
	}

	inner, err := rembed.Load(model, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load rembed model %q: %w", model, err)
	}

	return &RembedEmbedder{
		inner:       inner,
		modelName:   model,
		queryPrefix: cfg.QueryPrefix,
		docPrefix:   cfg.DocPrefix,
	}, nil
}

// NewEmbedderFromConfig constructs an Embedder from configuration, returning nil if disabled.
func NewEmbedderFromConfig(cfg config.EmbeddingsConfig) (Embedder, error) {
	if !cfg.Enabled || strings.EqualFold(strings.TrimSpace(cfg.Provider), "none") {
		return nil, nil
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" || provider == "rembed" {
		return NewRembedEmbedder(cfg)
	}
	return nil, fmt.Errorf("unsupported embeddings provider: %s", cfg.Provider)
}

// Embed computes L2-normalized document embeddings, applying DocPrefix when configured.
func (r *RembedEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	inputs := texts
	if r.docPrefix != "" {
		inputs = make([]string, len(texts))
		for i, t := range texts {
			inputs[i] = r.docPrefix + t
		}
	}
	vecs, err := r.inner.Embed(ctx, inputs)
	if err != nil {
		return nil, err
	}
	out := make([][]float32, len(vecs))
	for i, v := range vecs {
		out[i] = normalizeL2(v)
	}
	return out, nil
}

// EmbedQuery computes an L2-normalized query embedding, applying QueryPrefix when configured.
func (r *RembedEmbedder) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	input := query
	if r.queryPrefix != "" {
		input = r.queryPrefix + query
	}
	vecs, err := r.inner.Embed(ctx, []string{input})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, nil
	}
	return normalizeL2(vecs[0]), nil
}

// Model returns the configured model identifier.
func (r *RembedEmbedder) Model() string {
	return r.modelName
}

// Dimensions returns the embedding vector dimensionality.
func (r *RembedEmbedder) Dimensions() int {
	return r.inner.Dim()
}

// Close releases resources held by the underlying rembed embedder.
func (r *RembedEmbedder) Close() error {
	if r.inner == nil {
		return nil
	}
	return r.inner.Close()
}
