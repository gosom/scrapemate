//go:build !rod

package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gosom/scrapemate/adapters/fetchers/jshttp"
)

func main() {
	p, err := jshttp.StartAuthProxy(
		"socks5://isp.decodo.com:10001",
		"user",
		"pass",
	)
	if err != nil {
		panic(err)
	}

	defer p.Close()

	client := p.HTTPClient()

	for i := range 10 {
		func() {
			var schem string
			if i%2 == 0 {
				schem = "http"
			} else {
				schem = "https"
			}

			req, err := http.NewRequest(http.MethodGet, schem+"://httpbin.org/ip", http.NoBody)
			if err != nil {
				panic(err)
			}

			resp, err := client.Do(req)
			if err != nil {
				panic(err)
			}

			defer resp.Body.Close()

			fmt.Println("Response status:", resp.Status)

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				panic(err)
			}

			fmt.Println("Response body:", string(body))
		}()
	}
}
