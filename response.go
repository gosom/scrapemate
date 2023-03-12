package scrapemate

import (
	"bytes"
	"net/http"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Document struct {
	*goquery.Document
}

type Response struct {
	StatusCode int
	Headers    http.Header
	Duration   time.Duration
	Data       []byte
	Error      error
	Meta       map[string]any

	Document *Document
}

func (o *Response) SetDocument() error {
	o.Document = &Document{}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(o.Data))
	if err != nil {
		return err
	}
	o.Document.Document = doc
	return nil
}
