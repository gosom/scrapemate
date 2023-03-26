package goqueryparser

import (
	"bytes"
	"context"

	"github.com/PuerkitoBio/goquery"
)

type GoQueryHtmlParser struct {
}

func New() *GoQueryHtmlParser {
	return &GoQueryHtmlParser{}
}

func (g *GoQueryHtmlParser) Parse(ctx context.Context, body []byte) (any, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return doc, nil
}
