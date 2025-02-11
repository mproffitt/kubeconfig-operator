package helpers

import (
	"net"
	"net/url"
)

func AddressToSchemeHostPort(address string) (scheme, host, port string, err error) {
	var u *url.URL
	u, err = url.ParseRequestURI(address)
	if err != nil {
		return
	}
	scheme = u.Scheme
	if scheme == "" {
		scheme = "https"
	}

	host, port, err = net.SplitHostPort(u.Host)
	if err != nil {
		return
	}

	if host == "" {
		host = "localhost"
	}

	if port == "" {
		port = "6443"
	}

	return
}
