## Proxy Support

Scrapemate has builtin proxy support. 
You can add your proxies and each request will be fetched via the proxy.

When more that one proxy is provided they will be used in a round robin way.

To use proxies add the option `scrapemateapp.WithProxies()` to the config constructor.

example:

```go
proxies := []string{
	 "socks5://localhost:9050",
	 "http://user:pass@localhost:9051,
}

cfg, err := scrapemateapp.NewConfig(
	writers,
	scrapemateapp.WithConcurrency(1),
	scrapemateapp.WithProxies(proxies),
	scrapemateapp.WithExitOnInactivity(time.Minute)
)
```
