package selecthost

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var (
	// ErrSelectHost is the root error for all SelectHost errors
	ErrSelectHost = errors.New("SelectHost Error")
	// ErrSelectorStopped is returned when waiting for a host but the selector has been stopped.
	ErrSelectorStopped = fmt.Errorf("%w: selector stopped", ErrSelectHost)
	// ErrHostNotFound is returned when a host is not able to be determined.
	ErrHostNotFound = fmt.Errorf("%w: no host found", ErrSelectHost)
	// ErrWaitTimedout is returned when waiting for a host to be discovered.
	ErrWaitTimedout = fmt.Errorf("%w: timed out waiting for host", ErrSelectHost)
	// ErrDiscoveryTimeout is returned when the discovery process takes longer than configured timeout.
	ErrDiscoveryTimeout = fmt.Errorf("%w: discovery process timed out: %w", ErrSelectHost, context.DeadlineExceeded)
	// ErrHostCheckTimedout is returned when the host check process takes longer than configured timeout.
	ErrHostCheckTimedout = fmt.Errorf("%w: host check timed out: %w", ErrSelectHost, context.DeadlineExceeded)

	tracerName = "go.infratographer.com/iam-runtime-infratographer/internal/selecthost"
	tracer     = otel.GetTracerProvider().Tracer(tracerName)
)

// Selector handles discovering SRV records, periodically polling and selecting
// the fastest responding endpoint.
type Selector struct {
	logger *zap.SugaredLogger

	service  string
	protocol string
	target   string

	resolver interface {
		LookupSRV(ctx context.Context, service, protocol, target string) (string, []*net.SRV, error)
	}
	discoveryInterval time.Duration
	discoveryTimeout  time.Duration

	checkScheme      string
	checkPath        string
	checkCount       int
	checkInterval    time.Duration
	checkDelay       time.Duration
	checkTimeout     time.Duration
	checkConcurrency int

	initTimeout time.Duration

	mu        sync.RWMutex
	startOnce sync.Once
	quick     bool
	optional  bool

	stickyUntil time.Time
	selected    Host
	prefer      Host
	fallback    Host

	hosts Hosts

	checkOnce           sync.Once
	optionalFailureOnce sync.Once

	runCh     chan struct{}
	startWait chan struct{}
}

// Target returns the SRV target.
func (s *Selector) Target() string {
	return s.target
}

func (s *Selector) getHost() Host {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.selected
}

// GetHost returns the active host.
// If the selector has not been started before, the selector is initialized.
// This method will block until a host is selected, initialization timeout is reached or the context is canceled.
func (s *Selector) GetHost(ctx context.Context) (Host, error) {
	s.start(ctx)

	if host := s.getHost(); host != nil {
		return host, nil
	}

	ctx, cancel := context.WithTimeout(ctx, s.initTimeout)
	defer cancel()

	select {
	case <-s.runCh:
		return nil, ErrSelectorStopped
	case <-ctx.Done():
		return nil, ErrWaitTimedout
	case <-s.startWait:
		host := s.getHost()
		if host == nil {
			return nil, ErrHostNotFound
		}

		return host, nil
	}
}

func (s *Selector) start(ctx context.Context) {
	s.startOnce.Do(func() {
		ctx, span := tracer.Start(ctx, "selector.start", trace.WithAttributes(
			attribute.String("selector.target", s.target),
			attribute.String("selector.service", s.service),
			attribute.String("selector.protocol", s.protocol),
			attribute.Bool("selector.quick", s.quick),
		))
		defer span.End()

		s.logger.Infof("Initializing host selector for '%s'", s.target)

		if s.quick && s.fallback != nil {
			s.mu.Lock()
			s.selected = s.fallback
			s.mu.Unlock()

			span.AddEvent("Quick host selector enabled, selecting fallback address '" + s.fallback.ID() + "'")

			s.logger.Infof("Quick host selector enabled, selected fallback address '%s'", s.fallback.ID())

			// Start a new context keeping the current span so canceled contexts don't propagate.
			ctx = trace.ContextWithSpan(context.Background(), span)

			go s.discoverRecords(ctx)
		} else {
			s.discoverRecords(ctx)
		}

		go s.discovery()
	})
}

// Start initializes the selector discovery and checking handlers.
func (s *Selector) Start() {
	s.start(context.Background())
}

// Stop cleans up the service.
func (s *Selector) Stop() {
	close(s.runCh)
}

// selectHost updates the selected host based on the latest host list.
// Selection is made in the following order.
//
// 1. Select to the current host if found, without errors and within sticky period.
// 2. Select the preferred host if found and without errors.
// 3. Select the first host without errors.
// 4. Select the fallback host if configured.
// 5. No change to selected host.
//
// If the last step is reached and no host had previously been selected, the selected host is nil.
func (s *Selector) selectHost(ctx context.Context) {
	_, span := tracer.Start(ctx, "selectHost", trace.WithAttributes(
		attribute.Bool("host.changed", false),
	))
	defer span.End()

	s.mu.Lock()
	defer s.mu.Unlock()

	current := s.selected

	var (
		selected Host
		first    Host
		prefer   Host
	)

	for _, host := range s.hosts {
		if host.Err() != nil {
			continue
		}

		if first == nil {
			first = host
		}

		if selected == nil && s.selected != nil && s.selected.ID() == host.ID() {
			selected = host
		}

		if prefer == nil && s.prefer != nil && s.prefer.ID() == host.ID() {
			prefer = host
		}

		if first != nil && selected != nil && prefer != nil {
			break
		}
	}

	sticky := time.Now().Before(s.stickyUntil)

	switch {
	case selected != nil && sticky:
	case prefer != nil:
		selected = prefer
	case first != nil:
		selected = first
	case s.fallback != nil:
		selected = s.fallback
	}

	if current != nil {
		span.SetAttributes(
			attribute.String("host.current.id", current.ID()),
			attribute.Float64("host.current.avg_duration_ms", toMilliseconds(current.AverageDuration())),
			attribute.Bool("host.current.sticky", sticky),
		)

		if err := current.Err(); err != nil {
			span.SetAttributes(
				attribute.String("host.current.error", err.Error()),
			)
		}
	}

	if selected != nil {
		span.SetAttributes(
			attribute.String("host.selected.id", selected.ID()),
			attribute.Float64("host.selected.avg_duration_ms", toMilliseconds(selected.AverageDuration())),
		)

		if err := selected.Err(); err != nil {
			span.SetAttributes(
				attribute.String("host.selected.error", err.Error()),
			)
		}
	}

	if current != selected && selected != nil {
		s.selected = selected

		span.SetAttributes(attribute.Bool("host.changed", true))

		// ensure host doesn't change for 5 check intervals (as long as it's still healthy)
		s.stickyUntil = time.Now().Add(5 * s.checkInterval)

		if current == nil {
			span.AddEvent("selected host: " + selected.ID())

			s.logger.Infow("Host Selected",
				"selected.host", selected.ID(),
				"selected.check_duration_ms", toMilliseconds(selected.AverageDuration()),
			)
		} else {
			span.AddEvent("host changed: " + current.ID() + " -> " + selected.ID())

			s.logger.Warnw("Host Changed",
				"previous.host", current.ID(),
				"previous.check_duration_ms", toMilliseconds(current.AverageDuration()),
				"selected.host", selected.ID(),
				"selected.check_duration_ms", toMilliseconds(selected.AverageDuration()),
			)
		}
	} else if selected != nil && current == selected && !sticky {
		// If host remains the same but stickiness has expired, reset the sticky counter.
		s.stickyUntil = time.Now().Add(5 * s.checkInterval)
	}

	if selected == nil {
		currentID := ""
		if current != nil {
			currentID = current.ID()
		}

		span.SetStatus(codes.Error, "unable to select host")

		s.logger.Errorw("Unable to select host", "selected.host", currentID)
	}
}

func (s *Selector) discovery() {
	ticker := time.NewTicker(s.discoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.runCh:
			return
		case <-ticker.C:
		}

		s.discoverRecords(context.Background())
	}
}

func (s *Selector) discoverRecords(ctx context.Context) {
	target := s.target

	if strings.Contains(target, ":") {
		target, _, _ = net.SplitHostPort(target)
	}

	ctx, span := tracer.Start(ctx, "discoverRecords", trace.WithAttributes(
		attribute.String("discover.target", target),
		attribute.String("discover.service", s.service),
		attribute.String("discover.protocol", s.protocol),
	))
	defer span.End()

	origCtx := ctx

	ctx, cancel := context.WithTimeout(ctx, s.discoveryTimeout)
	defer cancel()

	start := time.Now()

	logger := s.logger.With(
		"discover.target", target,
		"discover.service", s.service,
		"discover.protocol", s.protocol,
	)

	logger.Debugf("Looking for srv records for service '%s' with protocol '%s' for target '%s'", s.service, s.protocol, target)

	cname, srvs, err := s.resolver.LookupSRV(ctx, s.service, s.protocol, target)

	span.SetAttributes(attribute.String("discover.resolved.cname", cname))

	duration := time.Since(start)

	logger = logger.With(
		"discover.resolved.cname", cname,
		"discover.runtime_ms", toMilliseconds(duration),
	)

	if err != nil {
		span.RecordError(err)

		if dnsErr, ok := err.(*net.DNSError); ok && dnsErr.IsNotFound && s.optional {
			s.optionalFailureOnce.Do(func() {
				span.AddEvent("no srv records found, however records are optional")

				logger.Warnw("No SRV records found, using target/fallback.")
			})
		} else {
			span.SetStatus(codes.Error, "Failed to lookup SRV records: "+err.Error())

			logger.Errorw("Failed to lookup SRV records", "error", err)
		}
	}

	s.mu.Lock()

	added, removed, matched := diffHosts(s, srvs)

	s.hosts = matched

	s.mu.Unlock()

	for _, host := range added {
		span.AddEvent("discovered " + host.ID())

		logger.Infof("Discovered host '%s'", host.ID())
	}

	for _, host := range removed {
		span.AddEvent("removed " + host.ID())

		logger.Warnf("Host removed '%s'", host.ID())
	}

	s.checkOnce.Do(func() {
		span.AddEvent("initializing host checks")

		s.checkHosts(origCtx)

		close(s.startWait)

		go s.watchHosts()
	})
}

func (s *Selector) watchHosts() {
	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.runCh:
			return
		case <-ticker.C:
		}

		s.checkHosts(context.Background())
	}
}

func (s *Selector) checkHosts(ctx context.Context) {
	ctx, span := tracer.Start(ctx, "checkHosts")
	defer span.End()

	s.mu.RLock()

	span.SetAttributes(
		attribute.Int("check.hosts.count", len(s.hosts)),
		attribute.Int("check.concurrency", s.checkConcurrency),
	)

	if len(s.hosts) == 0 {
		s.mu.RUnlock()

		s.selectHost(ctx)

		return
	}

	hostCh := make(chan Host, len(s.hosts))

	for _, host := range s.hosts {
		hostCh <- host
	}

	s.mu.RUnlock()

	close(hostCh)

	var wg sync.WaitGroup

	for i := 0; i < s.checkConcurrency; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for host := range hostCh {
				host.check(ctx)
			}
		}()
	}

	wg.Wait()

	s.mu.Lock()
	sort.Sort(s.hosts)
	s.mu.Unlock()

	s.selectHost(ctx)
}

// NewSelector creates a new selector service handler.
// The target provided is automatically registered as the default fallback address.
func NewSelector(target, service, protocol string, options ...Option) (*Selector, error) {
	sel := &Selector{
		logger: zap.NewNop().Sugar(),

		service:  service,
		protocol: protocol,
		target:   target,

		resolver:          net.DefaultResolver,
		discoveryInterval: 15 * time.Minute,
		discoveryTimeout:  2 * time.Second,

		checkCount:       5,
		checkInterval:    time.Minute,
		checkDelay:       200 * time.Millisecond,
		checkTimeout:     2 * time.Second,
		checkConcurrency: 5,

		initTimeout: 10 * time.Second,

		runCh:     make(chan struct{}),
		startWait: make(chan struct{}),
	}

	if err := Fallback(target)(sel); err != nil {
		return nil, err
	}

	for _, opt := range options {
		if err := opt(sel); err != nil {
			return nil, err
		}
	}

	return sel, nil
}
