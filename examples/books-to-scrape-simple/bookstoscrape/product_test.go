package bookstoscrape

import (
	"bytes"
	"os"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/require"
)

func Test_parseProduct(t *testing.T) {
	fileContents, err := os.ReadFile("../testdata/product.html")
	require.NoError(t, err)

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(fileContents))
	require.NoError(t, err)

	product, err := parseProduct(doc)
	require.NoError(t, err)

	require.Equal(t, "Scott Pilgrim's Precious Little Life (Scott Pilgrim #1)", product.Name)

	require.Equal(t, "Books", product.ProductType)
	require.Equal(t, "3b1c02bac2a429e6", product.UPC)
	require.Equal(t, "£", product.Currency)
	require.Equal(t, 52.29, product.PriceExclTax)
	require.Equal(t, 52.29, product.PriceInclTax)
	require.Equal(t, 0.0, product.Tax)
	require.Equal(t, true, product.InStock)
	require.Equal(t, 19, product.Availability)
	require.Equal(t, 0, product.NumbersOfReviews)
}
