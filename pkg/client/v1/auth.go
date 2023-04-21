// Package permissions defines the permissions-api client
package permissions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracer = otel.Tracer("go.infratographer.com/permissions-api/pkg/permissions/v1")

	appName    = "permissions-api"
	apiVersion = "/api/v1"
)

// Doer is an interface for an HTTP client that can make requests
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Client is used to interact with the api
type Client struct {
	url        string
	httpClient Doer
}

// New returns a new permissions client
func New(url string, doerClient Doer) (*Client, error) {
	if url == "" {
		return nil, ErrMissingURI
	}

	url = strings.TrimSuffix(url, "/")

	c := &Client{
		url: url,
	}

	c.httpClient = doerClient
	if c.httpClient == nil {
		// Use the default client as a fallback if one isn't passed
		c.httpClient = &http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		}
	}

	return c, nil
}

// Allowed checks if the client subject is permitted exec the action on the resource
func (c *Client) Allowed(ctx context.Context, action string, resourceURN string) (bool, error) {
	ctx, span := tracer.Start(ctx, "SubjectHasAction", trace.WithAttributes(
		attribute.String("action", action),
		attribute.String("resource", resourceURN),
	))
	defer span.End()

	values := url.Values{}
	values.Add("action", action)
	values.Add("resource", resourceURN)

	err := c.get(ctx, "/allow", values, map[string]string{})
	if err != nil {
		if errors.Is(err, ErrPermissionDenied) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// ServerResponse represents the data that the server will return on any given call
type ServerResponse struct {
	Message    string `json:"message,omitempty"`
	Error      string `json:"error,omitempty"`
	StatusCode int
}

func (c Client) get(ctx context.Context, endpoint string, query url.Values, resp interface{}) error {
	request, err := newGetRequest(ctx, c.url, endpoint, query)
	if err != nil {
		return err
	}

	return c.do(request, &resp)
}

func newGetRequest(ctx context.Context, uri, endpoint string, query url.Values) (*http.Request, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	u = u.JoinPath(apiVersion, endpoint)

	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}

	return http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
}

// userAgentString returns a string that identifies this client to the server
// TODO: This should be a constant, but we need to figure out how to get the version
// of the package at compile time
func userAgentString() string {
	return fmt.Sprintf("%s (%s)", appName, apiVersion)
}

func (c Client) do(req *http.Request, result interface{}) error {
	req.Header.Set("User-Agent", userAgentString())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	if err := ensureValidServerResponse(resp); err != nil {
		return err
	}

	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(&result)
}

func ensureValidServerResponse(resp *http.Response) error {
	if resp.StatusCode >= http.StatusMultiStatus {
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusForbidden {
			return ErrPermissionDenied
		}

		return ErrBadResponse
	}

	return nil
}
