package scrapemate

import (
	"fmt"
	"net/url"
	"strings"
)

// Proxy is a struct for proxy
type Proxy struct {
	URL      string
	Username string
	Password string
}

func NewProxy(u string) (Proxy, error) {
	if !strings.Contains(u, "://") {
		u = "socks5://" + u
	}

	pu, err := url.Parse(u)
	if err != nil {
		return Proxy{}, err
	}

	supportedSchemes := []string{"socks5", "http", "https"}

	scheme := strings.ToLower(pu.Scheme)

	var valid bool

	for _, s := range supportedSchemes {
		if s == scheme {
			valid = true

			break
		}
	}

	if !valid {
		return Proxy{}, fmt.Errorf("invalid proxy type: %s", scheme)
	}

	var username, password string
	if pu.User != nil {
		username = pu.User.Username()
		password, _ = pu.User.Password()
	}

	cleanURL := fmt.Sprintf("%s://%s", scheme, pu.Host)

	return Proxy{
		URL:      cleanURL,
		Username: username,
		Password: password,
	}, nil
}
