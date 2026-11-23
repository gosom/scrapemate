package jsonwriter

import (
	"bytes"
	"context"
	"testing"

	"github.com/gosom/scrapemate"
)

type place struct {
	Title string `json:"title"`
}

func TestRunWritesOneLinePerSliceItem(t *testing.T) {
	var buf bytes.Buffer

	in := make(chan scrapemate.Result, 2)
	in <- scrapemate.Result{Data: []*place{{Title: "a"}, {Title: "b"}}}
	in <- scrapemate.Result{Data: &place{Title: "c"}}
	close(in)

	if err := NewJSONWriter(&buf).Run(context.Background(), in); err != nil {
		t.Fatal(err)
	}

	want := "{\"title\":\"a\"}\n{\"title\":\"b\"}\n{\"title\":\"c\"}\n"
	if got := buf.String(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
