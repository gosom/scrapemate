package proxy

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
)

type Rotator struct {
	proxies  []string
	username string
	password string
	current  uint32
	cache    sync.Map
}

func New(proxies []string, username, password string) *Rotator {
	if len(proxies) == 0 {
		panic("no proxies provided")
	}

	return &Rotator{
		proxies:  proxies,
		username: username,
		password: password,
		current:  0,
	}
}

//nolint:gocritic // no need to change the signature
func (pr *Rotator) GetCredentials() (string, string) {
	return pr.username, pr.password
}

func (pr *Rotator) Next() string {
	current := atomic.AddUint32(&pr.current, 1)

	return pr.proxies[current%uint32(len(pr.proxies))] //nolint:gosec // no overflow here
}

func (pr *Rotator) RoundTrip(req *http.Request) (*http.Response, error) {
	proxyAddr := pr.Next()

	transport, ok := pr.cache.Load(proxyAddr)
	if !ok {
		proxyURL, err := url.Parse("socks5://" + proxyAddr)
		if err != nil {
			return nil, fmt.Errorf("error parsing proxy URL for %s: %v", proxyAddr, err)
		}

		if pr.username != "" && pr.password != "" {
			proxyURL.User = url.UserPassword(pr.username, pr.password)
		}

		transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}

		pr.cache.Store(proxyAddr, transport)
	}

	return transport.(*http.Transport).RoundTrip(req)
}
