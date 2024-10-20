package proxy

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"golang.org/x/net/proxy"
)

type Rotator struct {
	proxies []string
	current uint32
	cache   sync.Map
}

func New(proxies []string) *Rotator {
	if len(proxies) == 0 {
		panic("no proxies provided")
	}

	return &Rotator{
		proxies: proxies,
		current: 0,
	}
}

func (pr *Rotator) NextProxy() string {
	current := atomic.AddUint32(&pr.current, 1)

	return pr.proxies[current%uint32(len(pr.proxies))] //nolint:gosec // no overflow here
}

func (pr *Rotator) RoundTrip(req *http.Request) (*http.Response, error) {
	proxyAddr := pr.NextProxy()

	transport, ok := pr.cache.Load(proxyAddr)
	if !ok {
		dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("error creating SOCKS5 proxy dialer for %s: %v", proxyAddr, err)
		}

		transport = &http.Transport{
			Dial: dialer.Dial,
		}

		pr.cache.Store(proxyAddr, transport)
	}

	return transport.(*http.Transport).RoundTrip(req)
}
