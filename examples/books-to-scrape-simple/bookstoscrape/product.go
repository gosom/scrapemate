package bookstoscrape

import (
	"fmt"

	"github.com/PuerkitoBio/goquery"
)

type Product struct {
	Name             string
	UPC              string
	ProductType      string
	Currency         string
	PriceExclTax     float64
	PriceInclTax     float64
	Tax              float64
	InStock          bool
	Availability     int
	NumbersOfReviews int
	URL              string

	Screenshot []byte
}

func (o Product) CsvHeaders() []string {
	return []string{"name", "upc", "product_type", "currency", "price_excl_tax", "price_incl_tax", "tax", "in_stock", "availability", "numbers_of_reviews", "url"}
}

func (o Product) CsvRow() []string {
	return []string{o.Name, o.UPC, o.ProductType, o.Currency, fmt.Sprintf("%.2f", o.PriceExclTax), fmt.Sprintf("%.2f", o.PriceInclTax), fmt.Sprintf("%.2f", o.Tax), fmt.Sprintf("%t", o.InStock), fmt.Sprintf("%d", o.Availability), fmt.Sprintf("%d", o.NumbersOfReviews), o.URL}
}

func parseProduct(doc *goquery.Document) (product Product, err error) {
	product.Name = doc.Find("div.product_main>h1").Text()
	product.Currency = parseCurrency(doc.Find("div.product_main>p.price_color").Text())
	doc.Find("table.table-striped>tbody>tr").Each(func(i int, s *goquery.Selection) {
		switch s.Find("th").Text() {
		case "UPC":
			product.UPC = s.Find("td").Text()
		case "Product Type":
			product.ProductType = s.Find("td").Text()
		case "Price (excl. tax)":
			product.PriceExclTax = parsePrice(s.Find("td").Text())
		case "Price (incl. tax)":
			product.PriceInclTax = parsePrice(s.Find("td").Text())
		case "Tax":
			product.Tax = parsePrice(s.Find("td").Text())
		case "Availability":
			product.Availability = parseAvailability(s.Find("td").Text())
			product.InStock = product.Availability > 0
		case "Number of reviews":
			product.NumbersOfReviews = parseNumbeOfReviews(s.Find("td").Text())
		}
	})
	return product, nil
}

func parseCurrency(currency string) string {
	for _, c := range currency {
		return string(c)
	}
	return ""
}

func parseNumbeOfReviews(numberOfReviews string) int {
	var i int
	fmt.Sscanf(numberOfReviews, "%d", &i)
	return i
}

func parsePrice(price string) float64 {
	var f float64
	fmt.Sscanf(price, "£%f", &f)
	return f
}

func parseAvailability(availability string) int {
	var i int
	fmt.Sscanf(availability, "In stock (%d available)", &i)
	return i
}
