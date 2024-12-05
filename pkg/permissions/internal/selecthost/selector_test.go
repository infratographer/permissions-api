package selecthost

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestSelectorGetHost(t *testing.T) {
	t.Parallel()

	t.Run("immediate", func(t *testing.T) {
		t.Parallel()

		selector := &Selector{
			selected: &host{
				host: "host1.example.com",
			},
		}

		selector.startOnce.Do(func() {})

		host, err := selector.GetHost(context.Background())

		require.NoError(t, err, "no error expected to be returned")
		require.NotNil(t, host, "host expected to be returned")
		assert.Equal(t, "host1.example.com", host.Host(), "unexpected host returned")
	})

	t.Run("init timeout", func(t *testing.T) {
		t.Parallel()

		selector := &Selector{
			initTimeout: 10 * time.Millisecond,
			runCh:       make(chan struct{}),
			startWait:   make(chan struct{}),
		}

		defer close(selector.runCh)
		defer close(selector.startWait)

		selector.startOnce.Do(func() {})

		host, err := selector.GetHost(context.Background())

		require.Error(t, err, "error expected to be returned")
		require.Nil(t, host, "no host expected to be returned")

		assert.ErrorIs(t, err, ErrWaitTimedout, "unexpected error returned")
	})

	t.Run("selector shutdown", func(t *testing.T) {
		t.Parallel()

		selector := &Selector{
			initTimeout: 10 * time.Millisecond,
			runCh:       make(chan struct{}),
			startWait:   make(chan struct{}),
		}

		defer close(selector.startWait)

		selector.startOnce.Do(func() {})

		go func() {
			time.Sleep(2 * time.Millisecond)

			close(selector.runCh)
		}()

		host, err := selector.GetHost(context.Background())

		require.Error(t, err, "error expected to be returned")
		require.Nil(t, host, "no host expected to be returned")

		assert.ErrorIs(t, err, ErrSelectorStopped, "unexpected error returned")
	})

	t.Run("start finish without host", func(t *testing.T) {
		t.Parallel()

		selector := &Selector{
			initTimeout: 10 * time.Millisecond,
			runCh:       make(chan struct{}),
			startWait:   make(chan struct{}),
		}

		defer close(selector.runCh)

		selector.startOnce.Do(func() {})

		go func() {
			time.Sleep(2 * time.Millisecond)

			close(selector.startWait)
		}()

		host, err := selector.GetHost(context.Background())

		require.Error(t, err, "error expected to be returned")
		require.Nil(t, host, "no host expected to be returned")

		assert.ErrorIs(t, err, ErrHostNotFound, "unexpected error returned")
	})

	t.Run("start finish with host", func(t *testing.T) {
		t.Parallel()

		selector := &Selector{
			initTimeout: 10 * time.Millisecond,
			runCh:       make(chan struct{}),
			startWait:   make(chan struct{}),
		}

		defer close(selector.runCh)

		selector.startOnce.Do(func() {})

		go func() {
			time.Sleep(2 * time.Millisecond)

			selector.mu.Lock()
			defer selector.mu.Unlock()

			selector.selected = &host{
				host: "host1.example.com",
			}

			close(selector.startWait)
		}()

		host, err := selector.GetHost(context.Background())

		require.NoError(t, err, "no error expected to be returned")
		require.NotNil(t, host, "host expected to be returned")

		assert.Equal(t, "host1.example.com", host.Host(), "unexpected host returned")
	})
}

type testResolver struct {
	cname   string
	records []*net.SRV
	err     error

	requestedService  string
	requestedProtocol string
	requestedTarget   string
}

func (r *testResolver) LookupSRV(_ context.Context, service, protocol, target string) (string, []*net.SRV, error) {
	r.requestedService = service
	r.requestedProtocol = protocol
	r.requestedTarget = target

	return r.cname, r.records, r.err
}

func TestSelectorStart(t *testing.T) {
	t.Parallel()

	t.Run("quick", func(t *testing.T) {
		t.Parallel()

		selector := &Selector{
			logger: zap.NewNop().Sugar(),
			quick:  true,
			fallback: &host{
				host: "fallback.example.com",
			},
			resolver:          &testResolver{},
			discoveryInterval: time.Second,
			runCh:             make(chan struct{}),
		}

		close(selector.runCh)

		selector.checkOnce.Do(func() {})

		selector.start(context.Background())

		host := selector.getHost()

		require.NotNil(t, host, "expected host to not be nil")
		assert.Equal(t, "fallback.example.com", host.Host(), "unexpected host returned")
	})

	t.Run("not quick", func(t *testing.T) {
		t.Parallel()

		selector := &Selector{
			logger: zap.NewNop().Sugar(),
			quick:  false,
			fallback: &host{
				host: "fallback.example.com",
			},
			resolver:          &testResolver{},
			discoveryInterval: time.Second,
			runCh:             make(chan struct{}),
		}

		close(selector.runCh)

		selector.checkOnce.Do(func() {})

		selector.start(context.Background())

		host := selector.getHost()

		require.Nil(t, host, "expected host to be nil")
	})
}

// nolint: govet
func TestDiscoverRecords(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		target       string
		records      []*net.SRV
		expectTarget string
		expectHosts  []string
	}{
		{
			"with port",
			"iam.example.com:1234",
			[]*net.SRV{
				{"host1.example.com", 80, 0, 0},
			},
			"iam.example.com",
			[]string{
				"host1.example.com:80:0:0",
			},
		},
		{
			"without port",
			"iam.example.com",
			[]*net.SRV{
				{"host1.example.com", 80, 0, 0},
			},
			"iam.example.com",
			[]string{
				"host1.example.com:80:0:0",
			},
		},
		{
			"no records",
			"iam.example.com",
			[]*net.SRV{},
			"iam.example.com",
			[]string{},
		},
		{
			"priority changes",
			"iam.example.com",
			[]*net.SRV{
				{"new1.example.com", 80, 10, 10},
				{"old1.example.com", 80, 20, 10},
			},
			"iam.example.com",
			[]string{
				"new1.example.com:80:10:10",
				"old1.example.com:80:20:10",
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resolver := &testResolver{
				records: tc.records,
			}

			if len(tc.records) == 0 {
				resolver.err = &net.DNSError{
					IsNotFound: true,
				}
			}

			selector := &Selector{
				logger:   zap.NewNop().Sugar(),
				resolver: resolver,
				target:   tc.target,
			}

			selector.fallback = newHost(selector, "fallback.example.com", "80", net.SRV{})
			selector.hosts = Hosts{
				newHost(selector, "old1.example.com", "80", net.SRV{"old1.example.com", 80, 10, 10}),
			}

			selector.checkOnce.Do(func() {})

			selector.discoverRecords(context.Background())

			hosts := make([]string, len(selector.hosts))
			for i, host := range selector.hosts {
				hosts[i] = host.ID()
			}

			assert.Equal(t, tc.expectTarget, resolver.requestedTarget, "unexpected target queried")
			assert.Equal(t, tc.expectHosts, hosts, "unexpected hosts returned")
		})
	}
}

func TestSelectorSelectHost(t *testing.T) {
	t.Parallel()

	var hostErr = fmt.Errorf("%w: host error", errTestBase)

	type testHost struct {
		host string
		avg  time.Duration
		err  error
	}

	testCases := []struct {
		name           string
		hosts          []testHost
		current        string
		sticky         bool
		prefer         string
		fallback       string
		expectSelected string
	}{
		{
			"first healthy selection",
			[]testHost{
				{"host1.example.com", 10, hostErr},
				{"host2.example.com", 20, nil},
			},
			"",
			false,
			"",
			"",
			"host2.example.com",
		},
		{
			"current sticky",
			[]testHost{
				{"host1.example.com", 10, nil},
				{"host2.example.com", 20, nil},
			},
			"host2.example.com",
			true,
			"",
			"",
			"host2.example.com",
		},
		{
			"current not sticky",
			[]testHost{
				{"host1.example.com", 10, nil},
				{"host2.example.com", 20, nil},
			},
			"host2.example.com",
			false,
			"",
			"",
			"host1.example.com",
		},
		{
			"use preferred",
			[]testHost{
				{"host1.example.com", 10, nil},
				{"host2.example.com", 20, nil},
			},
			"host1.example.com",
			false,
			"host2.example.com",
			"",
			"host2.example.com",
		},
		{
			"fallback",
			[]testHost{
				{"host1.example.com", 10, hostErr},
				{"host2.example.com", 20, hostErr},
			},
			"host2.example.com",
			false,
			"",
			"host3.example.com",
			"host3.example.com",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			hosts := make(Hosts, len(tc.hosts))

			for i, h := range tc.hosts {
				hosts[i] = &host{
					id:   h.host,
					host: h.host,
					err:  h.err,
					lastCheck: Results{
						Checks:        1,
						TotalDuration: h.avg,
					},
				}
			}

			var selected, prefer, fallback Host

			if tc.current != "" {
				selected = &host{
					id:   tc.current,
					host: tc.current,
				}
			}

			if tc.prefer != "" {
				prefer = &host{
					id:   tc.prefer,
					host: tc.prefer,
				}
			}

			if tc.fallback != "" {
				fallback = &host{
					id:   tc.fallback,
					host: tc.fallback,
				}
			}

			selector := &Selector{
				logger: zap.NewNop().Sugar(),

				hosts:    hosts,
				selected: selected,
				prefer:   prefer,
				fallback: fallback,
			}

			if tc.sticky {
				selector.stickyUntil = time.Now().Add(time.Second)
			}

			selector.selectHost(context.Background())

			if tc.expectSelected == "" {
				require.Nil(t, selector.selected, "expected no host to be selected")

				return
			}

			require.NotNil(t, selector.selected, "expected a host to be selected")

			assert.Equal(t, tc.expectSelected, selector.selected.Host(), "unexpected host selected")
		})
	}
}

func TestSelectorCheckHosts(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		withHosts int
	}{
		{
			"no hosts",
			0,
		},
		{
			"multiple hosts",
			2,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			selector := &Selector{
				logger:           zap.NewNop().Sugar(),
				checkConcurrency: 2,
				checkCount:       5,
				checkTimeout:     time.Second,
				runCh:            make(chan struct{}),
			}

			defer close(selector.runCh)

			var checkCount atomic.Uint32

			hosts := make(Hosts, tc.withHosts)

			for i := 0; i < tc.withHosts; i++ {
				srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
					checkCount.Add(1)
				}))
				defer srv.Close()

				h, p, err := net.SplitHostPort(srv.Listener.Addr().String())
				require.NoError(t, err, "no error expected splitting test server address")

				hosts[i] = &host{
					selector: selector,
					id:       h + ":" + p,
					host:     h,
					port:     p,
				}
			}

			selector.hosts = hosts

			selector.checkHosts(context.Background())

			assert.Equal(t, 5*tc.withHosts, int(checkCount.Load()), "unexpected number of requests fetched")

			if tc.withHosts != 0 {
				assert.NotNil(t, selector.selected, "expected a host to be selected")
			} else {
				assert.Nil(t, selector.selected, "no host expected to be selected")
			}
		})
	}
}
