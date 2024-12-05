package selecthost

import (
	"fmt"
	"net"
	"net/http"
)

var _ http.RoundTripper = (*Transport)(nil)

// Transport implements http.RoundTripper handles switching the request host to a discovered host.
type Transport struct {
	Selector *Selector
	Base     http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
// If the request host matches the selector service's target, the host is replaced with the selected host address.
// If the request does not match, the base transport is called for the request instead.
//
// When the selected host is used, if the result from the base transport returns an error,
// the selected host is marked as having that error and a new host is immediately selected.
// The request however is not retried, instead the requestor must retry when appropriate.
//
// Hosts marked with an error will get cleared upon the next successful host check cycle.
func (t *Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	basert := t.Base
	if basert == nil {
		basert = http.DefaultTransport
	}

	host, err := t.Selector.GetHost(r.Context())
	if err != nil {
		return nil, err
	}

	if r.URL.Host != t.Selector.Target() {
		return basert.RoundTrip(r)
	}

	port := host.Port()

	// If no port was defined on the selected host, use the same port as the request, if one exists.
	if port == "" {
		_, port, _ = net.SplitHostPort(r.URL.Host)
	}

	// Remove default ports from host
	if (r.URL.Scheme == "http" && port == "80") || (r.URL.Scheme == "https" && port == "443") {
		port = ""
	}

	addr := JoinHostPort(host.Host(), port)

	r = r.Clone(r.Context())

	r.URL.Host = addr
	r.Host = addr

	resp, err := basert.RoundTrip(r)
	if err != nil {
		host.setError(err)
		t.Selector.selectHost(r.Context())

		return resp, fmt.Errorf("selected host: '%s': %w", addr, err)
	}

	return resp, nil
}

// NewTransport initialized a new Transport with the provided selector and base transport.
// If base is nil, the default http transport is used.
func NewTransport(selector *Selector, base http.RoundTripper) http.RoundTripper {
	return &Transport{
		Selector: selector,
		Base:     base,
	}
}
