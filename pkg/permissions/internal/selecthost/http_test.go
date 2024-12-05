package selecthost

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var errTestBase = errors.New("base error")

type testTransport struct {
	req *http.Request
	err error
}

func (t *testTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.req = r

	resp := &http.Response{
		Request: r,
		Body:    io.NopCloser(&bytes.Buffer{}),
	}

	return resp, t.err
}

func TestTransportRoundTrip(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		requestURL       string
		baseError        bool
		expectRequestURL string
		expectError      bool
		expectSelected   string
	}{
		{
			"success",
			"http://host.example.com/test-path",
			false,
			"http://host1.example.com/test-path",
			false,
			"host1.example.com",
		},
		{
			"failed",
			"http://host.example.com/test-path",
			true,
			"",
			true,
			"fallback.example.com",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, tc.requestURL, nil)
			require.NoError(t, err, "no error expected creating request")

			selector := &Selector{
				logger:    zap.NewNop().Sugar(),
				target:    "host.example.com",
				runCh:     make(chan struct{}),
				startWait: make(chan struct{}),
			}

			selector.startOnce.Do(func() {})

			defer close(selector.runCh)

			selected := &host{
				selector: selector,
				host:     "host1.example.com",
			}
			fallback := &host{
				selector: selector,
				host:     "fallback.example.com",
			}

			selector.selected = selected
			selector.fallback = fallback

			baseTransport := &testTransport{}

			if tc.baseError {
				baseTransport.err = errTestBase
			}

			transport := &Transport{
				Selector: selector,
				Base:     baseTransport,
			}

			resp, err := transport.RoundTrip(req)

			if tc.expectError {
				require.Error(t, err, "expected error to be returned")

				assert.Equal(t, tc.expectSelected, selector.selected.Host(), "unexpected host selected")

				return
			}

			require.NoError(t, err, "no error expected to be returned")

			defer resp.Body.Close()

			assert.Equal(t, tc.expectRequestURL, resp.Request.URL.String(), "unexpected url requested")
			assert.Equal(t, tc.expectSelected, selector.selected.Host(), "unexpected host selected")
		})
	}
}
