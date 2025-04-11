package selecthost

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// nolint: govet
func TestNewHost(t *testing.T) {
	t.Parallel()

	record := net.SRV{"srv target", 10, 20, 30}

	hostI := newHost(&Selector{}, "target", "port", record)

	require.NotNil(t, hostI, "expected host")

	host, ok := hostI.(*host)

	require.True(t, ok, "expected new host to be of *host type")
	assert.Equal(t, "target:0:20:30", host.id, "unexpected id")
	assert.Equal(t, "target", host.host, "unexpected host")
	assert.Equal(t, "port", host.port, "unexpected port")
	assert.Equal(t, record, host.record, "unexpected record")
}

func TestHostBefore(t *testing.T) {
	t.Parallel()

	hostWithoutErr := testHost(nil, "zero", "80", 10, 10, nil, 10)
	hostLowWeight := testHost(nil, "one", "80", 10, 10, nil, 10)
	hostHighWeight := testHost(nil, "two", "80", 20, 10, nil, 10)
	hostLowPrio := testHost(nil, "three", "80", 10, 10, nil, 10)
	hostHighPrio := testHost(nil, "four", "80", 10, 20, nil, 10)
	hostLowAvg := testHost(nil, "five", "80", 10, 10, nil, 10)
	hostHighAvg := testHost(nil, "six", "80", 10, 10, nil, 20)

	hostWithErr := testHost(nil, "zero-err", "80", 10, 10, net.ErrClosed, 10)

	testCases := []struct {
		name         string
		left         Host
		right        Host
		expectBefore bool
	}{
		{"same", hostLowWeight, hostLowWeight, false},
		{"error not before", hostWithErr, hostWithoutErr, false},
		{"no error before error", hostWithoutErr, hostWithErr, true},
		{"both error not before", hostWithErr, hostWithErr, false},
		{"low priority first", hostLowPrio, hostHighPrio, true},
		{"high priority not before", hostHighPrio, hostLowPrio, false},
		{"equal priority not before", hostLowPrio, hostLowPrio, false},
		{"low weight first", hostLowWeight, hostHighWeight, true},
		{"high weight not before", hostHighWeight, hostLowWeight, false},
		{"equal weight not before", hostLowWeight, hostLowWeight, false},
		{"low avg first", hostLowAvg, hostHighAvg, true},
		{"high avg not before", hostHighAvg, hostLowAvg, false},
		{"equal avg not before", hostLowAvg, hostLowAvg, false},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := tc.left.Before(tc.right)

			assert.Equal(t, tc.expectBefore, result, "unexpected before result")
		})
	}
}

func TestHostBuildURI(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		host   *host
		expect string
	}{
		{
			"default http",
			&host{
				selector: &Selector{},
				host:     "1.2.3.4",
			},
			"http://1.2.3.4",
		},
		{
			"default http default port",
			&host{
				selector: &Selector{},
				host:     "1.2.3.4",
				port:     "80",
			},
			"http://1.2.3.4",
		},
		{
			"default http non standard port",
			&host{
				selector: &Selector{},
				host:     "1.2.3.4",
				port:     "8080",
			},
			"http://1.2.3.4:8080",
		},
		{
			"default https default port",
			&host{
				selector: &Selector{},
				host:     "1.2.3.4",
				port:     "443",
			},
			"https://1.2.3.4",
		},
		{
			"scheme http no port",
			&host{
				selector: &Selector{checkScheme: "http"},
				host:     "1.2.3.4",
				port:     "",
			},
			"http://1.2.3.4",
		},
		{
			"scheme http with port",
			&host{
				selector: &Selector{checkScheme: "http"},
				host:     "1.2.3.4",
				port:     "80",
			},
			"http://1.2.3.4",
		},
		{
			"scheme http alt port",
			&host{
				selector: &Selector{checkScheme: "http"},
				host:     "1.2.3.4",
				port:     "8080",
			},
			"http://1.2.3.4:8080",
		},
		{
			"scheme https no port",
			&host{
				selector: &Selector{checkScheme: "https"},
				host:     "1.2.3.4",
			},
			"https://1.2.3.4",
		},
		{
			"scheme https with port",
			&host{
				selector: &Selector{checkScheme: "https"},
				host:     "1.2.3.4",
				port:     "443",
			},
			"https://1.2.3.4",
		},
		{
			"scheme https alt port",
			&host{
				selector: &Selector{checkScheme: "https"},
				host:     "1.2.3.4",
				port:     "8443",
			},
			"https://1.2.3.4:8443",
		},
		{
			"with path",
			&host{
				selector: &Selector{checkPath: "some/endpoint"},
				host:     "1.2.3.4",
			},
			"http://1.2.3.4/some/endpoint",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := tc.host.buildURI()

			assert.Equal(t, tc.expect, result.String(), "unexpected URI built")
		})
	}
}

func TestHostCheck(t *testing.T) {
	t.Parallel()

	checkDelay := time.Millisecond

	testCases := []struct {
		name                string
		responseDelay       time.Duration
		cancelDelay         time.Duration
		expectTotalDuration time.Duration
		expectErrors        int
	}{
		{
			"success",
			0,
			0,
			5 * checkDelay,
			0,
		},
		{
			"canceled",
			10 * time.Millisecond,
			20 * time.Millisecond,
			20 * time.Millisecond,
			1,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				time.Sleep(tc.responseDelay)
			}))

			defer srv.Close()

			h, p, err := net.SplitHostPort(srv.Listener.Addr().String())
			require.NoError(t, err, "no error expected splitting test server address")

			host := &host{
				selector: &Selector{
					logger:       zap.NewNop().Sugar(),
					checkDelay:   checkDelay,
					checkCount:   5,
					checkTimeout: 100 * time.Millisecond,
					runCh:        make(chan struct{}),
				},
				host: h,
				port: p,
			}

			ctx, cancel := context.WithCancel(context.Background())

			defer cancel()

			if tc.cancelDelay != 0 {
				go func() {
					time.Sleep(tc.cancelDelay)

					cancel()
				}()
			}

			start := time.Now()

			host.check(ctx)

			totalDuration := time.Since(start)

			if tc.expectErrors != 0 {
				require.Error(t, host.err, "expected error")

				assert.Equalf(t, tc.expectErrors, len(host.lastCheck.Errors), "expected %d errors, got %d | errors: %s", tc.expectErrors, len(host.lastCheck.Errors), errors.Join(host.lastCheck.Errors...))
			} else {
				require.NoError(t, host.err, "no error expected")
			}

			diff := totalDuration - tc.expectTotalDuration
			assert.Truef(t, diff > 0 && diff < 10*time.Millisecond, "total duration unexpected. got: %s want: %s", totalDuration, tc.expectTotalDuration)
		})
	}
}

func TestHostRun(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name               string
		responseDelay      time.Duration
		cancelDelay        time.Duration
		responseStatusCode int
		expectError        error
	}{
		{
			"success",
			0,
			0,
			http.StatusOK,
			nil,
		},
		{
			"invalid response",
			0,
			0,
			http.StatusInternalServerError,
			ErrUnexpectedStatusCode,
		},
		{
			"canceled",
			500 * time.Millisecond,
			100 * time.Millisecond,
			0,
			context.Canceled,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				time.Sleep(tc.responseDelay)

				if tc.responseStatusCode != 0 {
					w.WriteHeader(tc.responseStatusCode)
				}
			}))

			defer srv.Close()

			h, p, err := net.SplitHostPort(srv.Listener.Addr().String())
			require.NoError(t, err, "no error expected splitting test server address")

			host := &host{
				selector: &Selector{},
				host:     h,
				port:     p,
			}

			ctx, cancel := context.WithCancel(context.Background())

			defer cancel()

			if tc.cancelDelay != 0 {
				go func() {
					time.Sleep(tc.cancelDelay)

					cancel()
				}()
			}

			duration, err := host.run(ctx, zap.NewNop().Sugar())

			if tc.expectError != nil {
				require.Error(t, err, "expected error")

				assert.ErrorIs(t, err, tc.expectError, "unexpected error returned")

				return
			}

			require.NoError(t, err, "no error expected")

			if tc.responseDelay != 0 {
				diff := duration - tc.responseDelay
				assert.True(t, diff > 0 && diff < 10*time.Millisecond, "expected duration to be near response delay")
			}
		})
	}
}

func testHost(selector *Selector, target, port string, weight, priority uint16, err error, average time.Duration) Host {
	uport, _ := strconv.ParseUint(port, 10, 16)

	record := net.SRV{
		Target:   target,
		Port:     uint16(uport),
		Weight:   weight,
		Priority: priority,
	}

	hostI := newHost(selector, target, port, record)

	host := hostI.(*host)

	host.err = err

	host.lastCheck = Results{
		Checks:        1,
		TotalDuration: average,
	}

	return host
}
