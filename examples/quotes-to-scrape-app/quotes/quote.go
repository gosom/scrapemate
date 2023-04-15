package quotes

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Quote struct {
	Author string
	Text   string
	Tags   []string
}

func (q Quote) CsvHeaders() []string {
	return []string{"author", "text", "tags"}
}

func (q Quote) CsvRow() []string {
	return []string{q.Author, q.Text, strings.Join(q.Tags, ",")}
}

func parseQuotes(doc *goquery.Document) ([]Quote, error) {
	var quotes []Quote
	doc.Find(".quote").Each(func(i int, s *goquery.Selection) {
		quote := Quote{
			Author: s.Find(".author").Text(),
			Text:   s.Find(".text").Text(),
		}
		s.Find(".tag").Each(func(i int, s *goquery.Selection) {
			quote.Tags = append(quote.Tags, s.Text())
		})
		quotes = append(quotes, quote)
	})
	return quotes, nil
}
