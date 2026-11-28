package scrape

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/url"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html/charset"
)

// Markdown requests a text/markdown artifact named "markdown". It uses the
// main landmark, a single article, or the body, and omits navigation and footers.
// In browser mode it reads the rendered DOM after any selector wait.
func Markdown() Output {
	return Output{name: "markdown", mediaType: "text/markdown", render: renderMarkdown}
}

func renderMarkdown(ctx context.Context, result *Result) ([]byte, error) {
	if !result.IsHTML() {
		return nil, errors.New("Markdown requires an HTML response")
	}

	var reader io.Reader

	switch {
	case result.renderedHTML != nil:
		reader = strings.NewReader(*result.renderedHTML)
	case len(result.Response.Body) == 0:
		return nil, nil
	default:
		decoded, err := charset.NewReader(bytes.NewReader(result.Response.Body), result.Response.Headers.Get("Content-Type"))
		if err != nil {
			return nil, err
		}

		reader = decoded
	}

	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, err
	}

	baseURL := documentBaseURL(doc, result.Response.URL)

	doc.Find("nav, footer, [role=navigation], [role=contentinfo], [hidden], [aria-hidden=true], script, style, template, noscript").Remove()

	root := doc.Find("main, [role=main]").First()
	if root.Length() == 0 {
		root = doc.Find("article")
		if root.Length() != 1 {
			root = doc.Find("body")
		}
	}

	if root.Length() == 0 {
		return nil, nil
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}

	conv := converter.NewConverter(converter.WithPlugins(
		base.NewBasePlugin(),
		commonmark.NewCommonmarkPlugin(),
		table.NewTablePlugin(),
	))

	data, err := conv.ConvertNode(root.Nodes[0], converter.WithContext(ctx), converter.WithDomain(baseURL))
	if err != nil || len(data) == 0 {
		return data, err
	}

	return append(data, '\n'), nil
}

func documentBaseURL(doc *goquery.Document, finalURL string) string {
	href, ok := doc.Find("base[href]").First().Attr("href")
	if !ok {
		return finalURL
	}

	page, err := url.Parse(finalURL)
	if err != nil {
		return finalURL
	}

	resolved, err := page.Parse(strings.TrimSpace(href))
	if err != nil {
		return finalURL
	}

	return resolved.String()
}
