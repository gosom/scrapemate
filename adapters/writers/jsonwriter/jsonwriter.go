package jsonwriter

import (
	"context"
	"encoding/json"
	"io"

	"github.com/gosom/scrapemate"
)

var _ scrapemate.ResultWriter = (*jsonWriter)(nil)

type jsonWriter struct {
	enc *json.Encoder
}

func NewJSONWriter(w io.Writer) scrapemate.ResultWriter {
	enc := json.NewEncoder(w)
	return &jsonWriter{enc: enc}
}

func (c *jsonWriter) Run(_ context.Context, in <-chan scrapemate.Result) error {
	for result := range in {
		items := asSlice(result.Data)

		for i := range items {
			if err := c.enc.Encode(items[i]); err != nil {
				return err
			}
		}
	}

	return nil
}

func asSlice(t any) []any {
	isSlice, ok := t.([]any)
	if ok {
		return isSlice
	}

	var elements []any

	elements = append(elements, t)

	return elements
}
