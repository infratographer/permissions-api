package selecthost

import (
	"time"

	"go.uber.org/zap"
)

// Option defines a selector option.
type Option func(s *Selector) error

// Logger sets the logger.
func Logger(logger *zap.SugaredLogger) Option {
	return func(s *Selector) error {
		if logger != nil {
			s.logger = logger
		}

		return nil
	}
}

// DiscoveryInterval specifies the interval at which SRV records will be rediscovered.
// Default: 15m
func DiscoveryInterval(interval time.Duration) Option {
	return func(s *Selector) error {
		s.discoveryInterval = interval

		return nil
	}
}

// Quick will select the fallback address immediately on startup instead of waiting
// for the discovery process to complete.
func Quick() Option {
	return func(s *Selector) error {
		s.quick = true

		return nil
	}
}

// Optional if no SRV record is found, the target (or fallback address) is used instead.
// The discovery process continues to run in the chance that SRV records are added at a later point.
func Optional() Option {
	return func(s *Selector) error {
		s.optional = true

		return nil
	}
}

// CheckScheme sets the uri scheme.
// Default is http unless discovered port is 443, https will be used then.
func CheckScheme(scheme string) Option {
	return func(s *Selector) error {
		s.checkScheme = scheme

		return nil
	}
}

// CheckPath sets the request path for checks.
func CheckPath(path string) Option {
	return func(s *Selector) error {
		s.checkPath = path

		return nil
	}
}

// CheckCount defines how many checks to run on an endpoint.
// Default: 5
func CheckCount(count int) Option {
	return func(s *Selector) error {
		s.checkCount = count

		return nil
	}
}

// CheckInterval specifies how frequently to run host checks.
// Default: 1m.
func CheckInterval(interval time.Duration) Option {
	return func(s *Selector) error {
		s.checkInterval = interval

		return nil
	}
}

// CheckDelay specifies how long to wait between subsequent checks for the same host.
// Default: 200ms
func CheckDelay(delay time.Duration) Option {
	return func(s *Selector) error {
		s.checkDelay = delay

		return nil
	}
}

// CheckTimeout defines the maximum time an individual check request can take.
// Default: 2s
func CheckTimeout(timeout time.Duration) Option {
	return func(s *Selector) error {
		s.checkTimeout = timeout

		return nil
	}
}

// CheckConcurrency defines the number of hosts which may be checked simultaneously.
// Default: 5
func CheckConcurrency(count int) Option {
	return func(s *Selector) error {
		s.checkConcurrency = count

		return nil
	}
}

// Prefer specifies a preferred host.
// If the host is not discovered or has an error it will not be used.
func Prefer(host string) Option {
	return func(s *Selector) error {
		h, err := ParseHost(s, host)
		if err != nil {
			return err
		}

		s.prefer = h

		return nil
	}
}

// Fallback specifies a fallback host if no hosts are discovered or all hosts are currently failing.
func Fallback(host string) Option {
	return func(s *Selector) error {
		h, err := ParseHost(s, host)
		if err != nil {
			return err
		}

		s.fallback = h

		return nil
	}
}
