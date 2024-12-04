package selecthost

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJoinHostPort(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		host   string
		port   string
		expect string
	}{
		{"dns host only", "example.com", "", "example.com"},
		{"dns with port", "example.com", "8080", "example.com:8080"},
		{"ipv4 host only", "1.2.3.4", "", "1.2.3.4"},
		{"ipv4 with port", "1.2.3.4", "8080", "1.2.3.4:8080"},
		{"ipv6 host only", "::1", "", "[::1]"},
		{"ipv6 withi port", "::1", "8080", "[::1]:8080"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := JoinHostPort(tc.host, tc.port)

			assert.Equal(t, tc.expect, result, "unexpected joined host and port")
		})
	}
}

func TestHostID(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		host   string
		port   uint16
		expect string
	}{
		{"host only", "example.com", 0, "example.com:0:0:0"},
		{"with port", "example.com", 8080, "example.com:8080:0:0"},
		{"with trailing period", "example.com.", 8080, "example.com:8080:0:0"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := hostID(tc.host, tc.port, 0, 0)

			assert.Equal(t, tc.expect, result, "unexpected host id")
		})
	}
}

func TestParseHost(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		host        string
		expectHost  string
		expectPort  string
		expectError string
	}{
		{"dns host only", "example.com", "example.com", "", ""},
		{"dns with port", "example.com:8080", "example.com", "8080", ""},
		{"invalid", "[1:2", "", "", "missing ']' in address"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			host, err := ParseHost(nil, tc.host)

			if tc.expectError != "" {
				require.Error(t, err, "error expected")

				assert.ErrorContains(t, err, tc.expectError, "unexpected error returned")

				return
			}

			require.NoError(t, err, "no error expected")
			require.NotNil(t, host, "host expected to be returned")

			assert.Equal(t, tc.expectHost, host.Host(), "unexpected host")
			assert.Equal(t, tc.expectPort, host.Port(), "unexpected host port")
		})
	}
}

// nolint: govet
func TestDiffHosts(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		startingHosts Hosts
		srvs          []*net.SRV
		expectAdded   []string
		expectRemoved []string
		expectMatched []string
	}{
		{
			"empty",
			nil,
			nil,
			[]string{}, []string{}, []string{},
		},
		{
			"all added",
			nil,
			[]*net.SRV{
				{"one.example.com", 0, 10, 20},
				{"two.example.com", 80, 10, 20},
			},
			[]string{"one.example.com:0:10:20", "two.example.com:80:10:20"},
			[]string{},
			[]string{"one.example.com:0:10:20", "two.example.com:80:10:20"},
		},
		{
			"all removed",
			Hosts{
				&host{id: "one.example.com:0:10:20"},
				&host{id: "two.example.com:80:10:20"},
			},
			[]*net.SRV{},
			[]string{},
			[]string{"one.example.com:0:10:20", "two.example.com:80:10:20"},
			[]string{},
		},
		{
			"some removed, some added",
			Hosts{
				&host{id: "one.example.com:0:10:20"},
				&host{id: "two.example.com:80:10:20"},
			},
			[]*net.SRV{
				{"one.example.com", 0, 10, 20},
				{"three.example.com", 8080, 10, 20},
			},
			[]string{"three.example.com:8080:10:20"},
			[]string{"two.example.com:80:10:20"},
			[]string{"one.example.com:0:10:20", "three.example.com:8080:10:20"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := &Selector{hosts: tc.startingHosts}

			added, removed, matched := diffHosts(s, tc.srvs)

			addedIDs := getHostIDs(added)
			removedIDs := getHostIDs(removed)
			matchedIDs := getHostIDs(matched)

			assert.Equal(t, tc.expectAdded, addedIDs, "unexpected added hosts")
			assert.Equal(t, tc.expectRemoved, removedIDs, "unexpected removed hosts")
			assert.Equal(t, tc.expectMatched, matchedIDs, "unexpected matched hosts")
		})
	}
}

func getHostIDs(hosts []Host) []string {
	ids := make([]string, len(hosts))

	for i, host := range hosts {
		ids[i] = host.ID()
	}

	return ids
}
