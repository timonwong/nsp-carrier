package host

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/timonwong/nsp-carrier/internal/dbi"
	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/transport"
)

var (
	ErrUnknownProfile = errors.New("unknown installer profile")
	ErrDeviceNotFound = errors.New("installer USB device not found")
)

type ProfileID string

const ProfileDBI ProfileID = "dbi"

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
	Profile   ProfileID                   `json:"profile"`
	SessionID string                      `json:"sessionId"`
	State     State                       `json:"state"`
	Progress  map[string]ProgressSnapshot `json:"progress"`
	Err       error                       `json:"-"`
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
	Profile   ProfileID
	Catalog   *files.Catalog
	Connector Connector
	Observe   func(Event)
}

type Runner struct {
	newSessionID  func() string
	pollInterval  time.Duration
	shutdownGrace time.Duration
}

func NewRunner() *Runner {
	return &Runner{
		newSessionID: uuid.NewString, pollInterval: time.Second,
		shutdownGrace: 2 * time.Second,
	}
}

func (r *Runner) Run(ctx context.Context, request Request) error {
	if request.Profile != ProfileDBI {
		return fmt.Errorf("%w: %q", ErrUnknownProfile, request.Profile)
	}
	if request.Catalog == nil || request.Connector == nil {
		return errors.New("host session requires a frozen catalog and USB connector")
	}
	if _, err := dbi.NewServer(request.Catalog, nil); err != nil {
		return err
	}
	emit := func(sessionID string, state State, progress *Progress, err error) {
		if request.Observe == nil {
			return
		}
		var snapshots map[string]ProgressSnapshot
		if progress != nil {
			terminal := state == StateCompleted || state == StateDisconnected || state == StateFailed || state == StateStopping
			snapshots = progress.Snapshots(terminal, state == StateFailed)
		}
		request.Observe(Event{
			Profile: request.Profile, SessionID: sessionID, State: state,
			Progress: snapshots, Err: err,
		})
	}

	for {
		emit("", StateWaitingForDevice, nil, nil)
		connection, err := request.Connector.Open(ctx)
		if errors.Is(err, ErrDeviceNotFound) {
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

		sessionID := r.newSessionID()
		progress := NewProgress(request.Catalog)
		server, err := dbi.NewServer(request.Catalog, progress)
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
		case ctx.Err() != nil:
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

func (r *Runner) serve(
	ctx context.Context,
	server *dbi.Server,
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
