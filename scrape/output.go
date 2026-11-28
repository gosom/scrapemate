package scrape

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

// Artifact is a named representation produced from a fetched page.
type Artifact struct {
	Name      string
	MediaType string
	Data      []byte
}

// Artifact returns a requested output if its conversion succeeded.
func (r *Result) Artifact(name string) (Artifact, bool) {
	a, ok := r.artifacts[name]

	return a, ok
}

// Output describes a representation to produce during Fetch.
type Output struct {
	name      string
	mediaType string
	render    func(context.Context, *Result) ([]byte, error)
}

// WithOutputs requests additional representations from the same navigation.
// Output names must be unique across all options for a fetch.
func WithOutputs(outputs ...Output) FetchOption {
	outputs = slices.Clone(outputs)

	return func(cfg *fetchConfig) error {
		for _, out := range outputs {
			if out.render == nil {
				return errors.New("scrape: invalid output")
			}

			if slices.ContainsFunc(cfg.outputs, func(o Output) bool { return o.name == out.name }) {
				return fmt.Errorf("scrape: duplicate output %q", out.name)
			}

			cfg.outputs = append(cfg.outputs, out)
		}

		return nil
	}
}

func (r *Result) produceOutputs(ctx context.Context, outputs []Output) error {
	r.artifacts = make(map[string]Artifact, len(outputs))

	for _, out := range outputs {
		data, err := out.render(ctx, r)
		if err != nil {
			return fmt.Errorf("produce %s: %w", out.name, err)
		}

		if err := ctx.Err(); err != nil {
			return fmt.Errorf("produce %s: %w", out.name, err)
		}

		r.artifacts[out.name] = Artifact{Name: out.name, MediaType: out.mediaType, Data: data}
	}

	return nil
}
