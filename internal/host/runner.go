package host

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/timonwong/nsp-carrier/internal/awoo"
	"github.com/timonwong/nsp-carrier/internal/dbi"
	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/goldleaf"
	"github.com/timonwong/nsp-carrier/internal/protocoltrace"
	"github.com/timonwong/nsp-carrier/internal/sphaira"
	"github.com/timonwong/nsp-carrier/internal/transport"
)

var (
	ErrUnknownProfile     = errors.New("unknown installer profile")
	ErrDeviceNotFound     = errors.New("installer USB device not found")
	ErrDeviceUnavailable  = errors.New("installer USB device temporarily unavailable")
	ErrProfileUnavailable = errors.New("installer profile adapter is not implemented")
)

type State string

const (
	StateWaitingForDevice State = "WaitingForDevice"
	StateConnected        State = "Connected"
	StateServing          State = "Serving"
	StateCompleted        State = "Completed"
	StateDisconnected     State = "Disconnected"
	StateFailed           State = "Failed"
	StateStopping         State = "Stopping"
)

type ProgressSnapshot struct {
	SourceID              string    `json:"sourceId"`
	State                 FileState `json:"state"`
	UniqueServedBytes     uint64    `json:"uniqueServedBytes"`
	WireBytes             uint64    `json:"wireBytes"`
	TotalBytes            uint64    `json:"totalBytes"`
	Percent               float64   `json:"percent"`
	RangeRequests         uint64    `json:"rangeRequests"`
	NonSequentialRequests uint64    `json:"nonSequentialRequests"`
	BackwardRequests      uint64    `json:"backwardRequests"`
	RepeatedRequests      uint64    `json:"repeatedRequests"`
	MaxRequestedOffset    uint64    `json:"maxRequestedOffset"`
}

type Event struct {
	Profile                ProfileID                   `json:"profile"`
	SessionID              string                      `json:"sessionId"`
	State                  State                       `json:"state"`
	Progress               map[string]ProgressSnapshot `json:"progress"`
	Warnings               []Warning                   `json:"warnings,omitempty"`
	ProtocolTrace          []protocoltrace.Record      `json:"protocolTrace,omitempty"`
	ProtocolTraceTruncated bool                        `json:"protocolTraceTruncated,omitempty"`
	Err                    error                       `json:"-"`
}

// SameState reports whether two events describe the same observable session
// state. Progress and warnings are snapshots delivered alongside that state;
// changes to them do not constitute a state transition.
func (e Event) SameState(other Event) bool {
	return e.Profile == other.Profile &&
		e.SessionID == other.SessionID &&
		e.State == other.State &&
		errorText(e.Err) == errorText(other.Err)
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

type Warning struct {
	Sequence  uint64 `json:"sequence"`
	Operation string `json:"operation"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

type warningLog struct {
	mu       sync.Mutex
	next     uint64
	warnings []Warning
}

func (l *warningLog) add(warning Warning) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.next++
	warning.Sequence = l.next
	l.warnings = append(l.warnings, warning)
	const maxSessionWarnings = 300
	if len(l.warnings) > maxSessionWarnings {
		copy(l.warnings, l.warnings[len(l.warnings)-maxSessionWarnings:])
		l.warnings = l.warnings[:maxSessionWarnings]
	}
}

func (l *warningLog) snapshot() []Warning {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]Warning(nil), l.warnings...)
}

type Connection interface {
	transport.Duplex
	Close() error
	Shutdown(context.Context) error
}

type Connector interface {
	Open(context.Context) (Connection, error)
}

type Request struct {
	Profile       ProfileID
	Catalog       *files.Catalog
	Connector     Connector
	TraceProtocol bool
	Observe       func(Event)
}

type Runner struct {
	newSessionID  func() string
	pollInterval  time.Duration
	shutdownGrace time.Duration
}

const maxRecoverableOpenFailures = 3

func NewRunner() *Runner {
	return &Runner{
		newSessionID: uuid.NewString, pollInterval: time.Second,
		shutdownGrace: 2 * time.Second,
	}
}

func (r *Runner) Run(ctx context.Context, request Request) error {
	profile, ok := ProfileByID(request.Profile)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownProfile, request.Profile)
	}
	if request.Catalog == nil || request.Connector == nil {
		return errors.New("host session requires a frozen catalog and USB connector")
	}
	validationErrors, err := ValidateCatalog(request.Profile, request.Catalog)
	if err != nil {
		return err
	}
	if len(validationErrors) > 0 {
		return CatalogValidationErrors(validationErrors)
	}
	if !profile.AdapterAvailable {
		return fmt.Errorf("%w: %s", ErrProfileUnavailable, request.Profile)
	}
	if _, err := newAdapter(request.Profile, request.Catalog, nil, nil, nil); err != nil {
		return err
	}
	var warnings *warningLog
	var trace *protocoltrace.Buffer
	emit := func(sessionID string, state State, progress *Progress, err error) {
		if request.Observe == nil {
			return
		}
		var snapshots map[string]ProgressSnapshot
		if progress != nil {
			terminal := state == StateCompleted || state == StateDisconnected || state == StateFailed || state == StateStopping
			snapshots = progress.Snapshots(terminal, state == StateFailed)
		}
		var warningSnapshot []Warning
		if warnings != nil {
			warningSnapshot = warnings.snapshot()
		}
		var traceSnapshot []protocoltrace.Record
		var traceTruncated bool
		if trace != nil {
			traceSnapshot, traceTruncated = trace.Snapshot()
		}
		request.Observe(Event{
			Profile: request.Profile, SessionID: sessionID, State: state,
			Progress: snapshots, Warnings: warningSnapshot,
			ProtocolTrace: traceSnapshot, ProtocolTraceTruncated: traceTruncated, Err: err,
		})
	}

	recoverableOpenFailures := 0
	for {
		warnings = nil
		trace = nil
		emit("", StateWaitingForDevice, nil, nil)
		connection, err := request.Connector.Open(ctx)
		if errors.Is(err, ErrDeviceUnavailable) {
			recoverableOpenFailures++
			if recoverableOpenFailures >= maxRecoverableOpenFailures {
				emit("", StateFailed, nil, err)
				return err
			}
		} else if errors.Is(err, ErrDeviceNotFound) {
			recoverableOpenFailures = 0
		}
		if errors.Is(err, ErrDeviceNotFound) || errors.Is(err, ErrDeviceUnavailable) {
			timer := time.NewTimer(r.pollInterval)
			select {
			case <-ctx.Done():
				timer.Stop()
				emit("", StateStopping, nil, ctx.Err())
				return ctx.Err()
			case <-timer.C:
				continue
			}
		}
		if err != nil {
			emit("", StateFailed, nil, err)
			return err
		}
		recoverableOpenFailures = 0

		sessionID := r.newSessionID()
		progress := NewProgress(request.Catalog, request.Profile)
		warnings = &warningLog{}
		if request.TraceProtocol {
			trace = &protocoltrace.Buffer{}
		}
		var traceReporter protocoltrace.Reporter
		if trace != nil {
			traceReporter = trace
		}
		server, err := newAdapter(request.Profile, request.Catalog, progress, warnings.add, traceReporter)
		if err != nil {
			_ = connection.Close()
			return err
		}
		emit(sessionID, StateConnected, progress, nil)
		emit(sessionID, StateServing, progress, nil)
		serveErr := r.serve(ctx, server, connection, sessionID, progress, emit)
		closeErr := connection.Close()

		switch {
		case serveErr == nil:
			emit(sessionID, StateCompleted, progress, closeErr)
			return closeErr
		case ctx.Err() != nil || errors.Is(serveErr, context.Canceled):
			err = errors.Join(ctx.Err(), serveErr, closeErr)
			emit(sessionID, StateStopping, progress, err)
			return err
		case errors.Is(serveErr, transport.ErrDisconnected):
			emit(sessionID, StateDisconnected, progress, serveErr)
			continue
		default:
			err = errors.Join(serveErr, closeErr)
			emit(sessionID, StateFailed, progress, err)
			return err
		}
	}
}

type protocolAdapter interface {
	Serve(context.Context, transport.Duplex) error
}

func newAdapter(
	profile ProfileID,
	catalog *files.Catalog,
	progress *Progress,
	warn func(Warning),
	trace protocoltrace.Reporter,
) (protocolAdapter, error) {
	switch profile {
	case ProfileDBI:
		return dbi.NewServer(catalog, progress)
	case ProfileAwoo:
		return awoo.NewServerWithTrace(catalog, progress, trace)
	case ProfileGoldleaf:
		return goldleaf.NewServerWithTrace(catalog, progress, func(protocolWarning goldleaf.Warning) {
			if warn != nil {
				warn(Warning{
					Operation: protocolWarning.Operation,
					Code:      protocolWarning.Code,
					Message:   protocolWarning.Message,
				})
			}
		}, trace)
	case ProfileSphaira:
		return sphaira.NewServerWithTrace(catalog, progress, trace)
	default:
		return nil, fmt.Errorf("%w: %s", ErrProfileUnavailable, profile)
	}
}

func (r *Runner) serve(
	ctx context.Context,
	server protocolAdapter,
	connection Connection,
	sessionID string,
	progress *Progress,
	emit func(string, State, *Progress, error),
) error {
	result := make(chan error, 1)
	go func() { result <- server.Serve(ctx, connection) }()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-result:
			return err
		case <-ticker.C:
			emit(sessionID, StateServing, progress, nil)
		case <-ctx.Done():
			return r.stop(ctx, result, connection)
		}
	}
}

func (r *Runner) stop(ctx context.Context, result <-chan error, connection Connection) error {
	grace := time.NewTimer(r.shutdownGrace)
	defer grace.Stop()
	select {
	case err := <-result:
		return err
	case <-grace.C:
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), r.shutdownGrace)
	defer cancel()
	if err := connection.Shutdown(shutdownCtx); err != nil {
		return errors.Join(ctx.Err(), fmt.Errorf("shutdown USB connection: %w", err))
	}
	select {
	case err := <-result:
		return errors.Join(ctx.Err(), err)
	case <-shutdownCtx.Done():
		return errors.New("USB session did not stop after transfer shutdown")
	}
}
