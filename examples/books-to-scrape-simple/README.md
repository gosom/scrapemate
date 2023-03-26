# Extract product information to csv from books.toscrape.com

This is an example use case of scrapemate. 

Here we are going to scrape all the books from [books.toscrape.com](https://books.toscrape.com/).

Our goal is to scrape for each product the following attributes:

```
name: the name of the product
UPC: a unique identifier
product_type: the product type
currency: the currency symbol of the price
price_exl_tax: price without tax
price_incl_tax: price including tax
tax: how much is tha tax
in_stock: true when product is available
availability: number of items in stock
price: the total price (including taxes)
number_of_reviews: the number of reviews the product has
```

We want to save them in a csv file with headers the column names.


###  How to run it

```
go run main.go 1>bookstoscrape.csv
```

The crawler does not have a mechanism to exit when crawling finishes, it justs sits there
and waits for new jobs.

You can exit the crawler via CTRL-C when there are no logs or when you have 1001 rows in the 
`bookstoscrape.csv` file.

