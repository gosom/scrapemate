package csvwriter_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gosom/scrapemate"
	"github.com/gosom/scrapemate/adapters/writers/csvwriter"
)

type item struct{ name string }

func (i item) CsvHeaders() []string { return []string{"name"} }
func (i item) CsvRow() []string     { return []string{i.name} }

// A job that only yields next jobs sends a result with no data.
func TestRunSkipsResultsWithoutData(t *testing.T) {
	var buf bytes.Buffer

	in := make(chan scrapemate.Result, 2)
	in <- scrapemate.Result{Data: nil}

	in <- scrapemate.Result{Data: item{name: "a"}}

	close(in)

	err := csvwriter.NewCsvWriter(csv.NewWriter(&buf)).Run(context.Background(), in)

	require.NoError(t, err)
	require.Equal(t, "name\na\n", buf.String())
}
