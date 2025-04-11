package selecthost

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-cleanhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var (
	// ErrUnexpectedStatusCode is returned when a check request doesn't have 2xx status code.
	ErrUnexpectedStatusCode = fmt.Errorf("%w: unexpected status code", ErrSelectHost)
)

// httpClient sets the client timeout to 10 seconds.
var httpClient = &http.Client{
	Transport: otelhttp.NewTransport(cleanhttp.DefaultPooledTransport()),
	Timeout:   10 * time.Second, //nolint:mnd
}

// Host is an individual host entry.
type Host interface {
	ID() string
	Record() net.SRV
	Host() string
	Port() string

	Before(h2 Host) bool

	check(ctx context.Context)
	LastCheck() Results
	AverageDuration() time.Duration
	Err() error
	setError(err error)
}

// Hosts is a collection of [Host]s.
type Hosts []Host

// Len implement sort.Interface.
func (h Hosts) Len() int { return len(h) }

// Less implements sort.Interface.
func (h Hosts) Less(i, j int) bool {
	return h[i].Before(h[j])
}

// Swap implement sort.Interface.
func (h Hosts) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func newHost(selector *Selector, target string, port string, srv net.SRV) Host {
	target = strings.TrimRight(target, ".")
	iport, _ := strconv.ParseUint(port, 10, 16) // no error check, if it's an invalid port we'll use the default 0

	return &host{
		selector: selector,
		id:       hostID(target, uint16(iport), srv.Priority, srv.Weight),
		record:   srv,
		host:     target,
		port:     port,
	}
}

type host struct {
	selector *Selector

	mu sync.RWMutex

	id     string
	record net.SRV
	host   string
	port   string

	lastCheck Results
	err       error
}

// ID returns the host ID.
func (h *host) ID() string {
	return h.id
}

// Record returns the SRV records.
func (h *host) Record() net.SRV {
	return h.record
}

// Host returns the host.
func (h *host) Host() string {
	return h.host
}

// Port returns the port.
func (h *host) Port() string {
	return h.port
}

// LastCheck returns the latest check results.
func (h *host) LastCheck() Results {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.lastCheck
}

// AverageDuration returns the latest check's average duration.
func (h *host) AverageDuration() time.Duration {
	return h.LastCheck().Average()
}

// Err returns the error recorded on the host.
func (h *host) Err() error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.err
}

func (h *host) setError(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.err = err
}

// Before compares if the left host is before the provided host.
func (h *host) Before(h2 Host) bool {
	switch {
	case h.Err() == nil && h2.Err() != nil:
		return true
	case h.Err() != nil && h2.Err() == nil:
		return false
	case h.Record().Priority < h2.Record().Priority:
		return true
	case h.Record().Priority > h2.Record().Priority:
		return false
	case h.Record().Weight < h2.Record().Weight:
		return true
	case h.Record().Weight > h2.Record().Weight:
		return false
	case h.AverageDuration() < h2.AverageDuration():
		return true
	}

	return false
}

func (h *host) check(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	ctx, span := tracer.Start(ctx, "host.check", trace.WithAttributes(
		attribute.String("check.host", h.id),
		attribute.Int("check.total", h.selector.checkCount),
		attribute.Float64("check.delay_ms", float64(h.selector.checkDelay)/float64(time.Millisecond)),
	))

	defer span.End()

	results := Results{
		Time: time.Now(),
		Host: h,
	}

	cancel := func() {}

	logger := h.selector.logger.With(
		"check.total", h.selector.checkCount,
		"check.host", h.id,
	)

	logger.Debugf("Starting host checks for '%s'", h.id)

checkLoop:
	for i := 0; i < h.selector.checkCount; i++ {
		if i != 0 {
			cancel()

			select {
			case <-ctx.Done():
				break checkLoop
			case <-h.selector.runCh:
				break checkLoop
			default:
			}

			time.Sleep(h.selector.checkDelay)
		}

		cctx, ccancel := context.WithTimeoutCause(ctx, h.selector.checkTimeout, ErrHostCheckTimedout)

		cancel = ccancel

		// If the manager is stopped, cancel the context, also exit if the context gets canceled.
		go func() {
			select {
			case <-cctx.Done():
			case <-h.selector.runCh:
				ccancel()
			}
		}()

		duration, err := h.run(cctx, logger.With("check.run", i))
		if err != nil {
			results.Errors = append(results.Errors, err)
		}

		results.Checks++
		results.TotalDuration += duration
	}

	cancel()

	span.SetAttributes(attribute.Float64("check.average_ms", toMilliseconds(results.Average())))

	logger = logger.With("check.average_ms", toMilliseconds(results.Average()))

	if len(results.Errors) != 0 {
		errs := errors.Join(results.Errors...)
		span.RecordError(errs)
		span.SetStatus(codes.Error, errs.Error())

		logger.Errorw("Host checks completed with errors", "errors", results.Errors)
	} else {
		logger.Debugf("Host checks completed without errors")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.lastCheck = results

	if len(results.Errors) != 0 {
		h.err = results.Errors[len(results.Errors)-1]
	} else {
		h.err = nil
	}
}

func (h *host) buildURI() *url.URL {
	scheme := "http"

	if h.port == "443" {
		scheme = "https"
	}

	if h.selector.checkScheme != "" {
		scheme = h.selector.checkScheme
	}

	port := h.port

	// Strip port for default scheme ports.
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}

	return &url.URL{
		Scheme: scheme,
		Host:   JoinHostPort(h.host, port),
		Path:   h.selector.checkPath,
	}
}

func (h *host) run(ctx context.Context, logger *zap.SugaredLogger) (time.Duration, error) {
	uri := h.buildURI()

	logger = logger.With(
		"check.uri", uri.String(),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri.String(), nil)
	if err != nil {
		logger.Errorw("Failed to create check request", "error", err)

		return 0, err
	}

	start := time.Now()

	resp, err := httpClient.Do(req)
	if err != nil {
		duration := time.Since(start)

		logger.Errorw("Failed to execute check request", "error", err, "check.duration_ms", toMilliseconds(duration))

		return duration, err
	}

	defer resp.Body.Close() //nolint:errcheck

	// Consume body so connection can be reused.
	// If an error occurs reading the body, ignore.
	body, _ := io.ReadAll(resp.Body)

	duration := time.Since(start)

	logger = logger.With(
		"check.duration_ms", toMilliseconds(duration),
		"check.response.status_code", resp.StatusCode,
	)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Errorw("Check completed with an unexpected status code", "error", err)

		return duration, fmt.Errorf("%w: %d: %s", ErrUnexpectedStatusCode, resp.StatusCode, string(body))
	}

	logger.Debug("Check completed successfully")

	return duration, nil
}

// Results holds host check results.
type Results struct {
	Time          time.Time
	Host          Host
	Checks        uint
	TotalDuration time.Duration
	Errors        []error
}

// Average returns the average duration of all checks run on host.
func (r Results) Average() time.Duration {
	if r.Checks == 0 {
		return 0
	}

	return r.TotalDuration / time.Duration(r.Checks)
}
