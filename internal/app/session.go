package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/gousb"
	"github.com/timonwong/nsp-carrier/internal/dbi"
	"github.com/timonwong/nsp-carrier/internal/files"
	usbtransport "github.com/timonwong/nsp-carrier/internal/usb"
)

const (
	devicePollInterval    = time.Second
	progressEventInterval = 100 * time.Millisecond
	shutdownGracePeriod   = 2 * time.Second
)

func runUSBSession(ctx context.Context, catalog *files.Catalog, update func(sessionUpdate)) error {
	for {
		connection, err := usbtransport.Open(usbtransport.OpenOptions{ResetOnConnect: false})
		if errors.Is(err, usbtransport.ErrDeviceNotFound) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(devicePollInterval):
				continue
			}
		}
		if err != nil {
			update(sessionUpdate{state: StateFailed, err: err, terminal: true})
			return err
		}

		sessionID := time.Now().UTC().Format("20060102T150405.000000000Z")
		update(sessionUpdate{state: StateConnected, sessionID: sessionID})
		server := dbi.NewServer(catalog)
		update(sessionUpdate{state: StateServing, sessionID: sessionID, progress: collectProgress(server, catalog)})
		serveErr := serveWithProgress(ctx, server, connection, catalog, sessionID, update)
		closeErr := connection.Close()
		progress := collectProgress(server, catalog)

		if serveErr == nil {
			update(sessionUpdate{state: StateCompleted, sessionID: sessionID, progress: progress, terminal: true})
			return closeErr
		}
		if ctx.Err() != nil {
			update(sessionUpdate{state: StateStopping, sessionID: sessionID, progress: progress, terminal: true})
			return ctx.Err()
		}
		if isDisconnect(serveErr) {
			update(sessionUpdate{state: StateDisconnected, sessionID: sessionID, progress: progress, terminal: true})
			continue
		}
		err = errors.Join(serveErr, closeErr)
		update(sessionUpdate{state: StateFailed, sessionID: sessionID, progress: progress, err: err, terminal: true})
		return err
	}
}

func serveWithProgress(
	ctx context.Context,
	server *dbi.Server,
	connection *usbtransport.Connection,
	catalog *files.Catalog,
	sessionID string,
	update func(sessionUpdate),
) error {
	result := make(chan error, 1)
	go func() { result <- server.Serve(ctx, connection) }()
	ticker := time.NewTicker(progressEventInterval)
	defer ticker.Stop()

	for {
		select {
		case err := <-result:
			return err
		case <-ticker.C:
			update(sessionUpdate{state: StateServing, sessionID: sessionID, progress: collectProgress(server, catalog)})
		case <-ctx.Done():
			return stopServing(ctx, result, connection)
		}
	}
}

func stopServing(ctx context.Context, result <-chan error, connection *usbtransport.Connection) error {
	grace := time.NewTimer(shutdownGracePeriod)
	defer grace.Stop()
	select {
	case err := <-result:
		return err
	case <-grace.C:
	}
	if err := connection.ForceReset(); err != nil {
		return errors.Join(ctx.Err(), fmt.Errorf("force reset: %w", err))
	}
	forced := time.NewTimer(shutdownGracePeriod)
	defer forced.Stop()
	select {
	case err := <-result:
		return errors.Join(ctx.Err(), err)
	case <-forced.C:
		return errors.New("USB session did not stop after cancellation and device reset")
	}
}

func collectProgress(server *dbi.Server, catalog *files.Catalog) map[string]dbi.ProgressSnapshot {
	result := make(map[string]dbi.ProgressSnapshot, len(catalog.Entries()))
	for _, entry := range catalog.Entries() {
		if progress, ok := server.Progress(entry.Name); ok {
			result[entry.Name] = progress
		}
	}
	return result
}

func isDisconnect(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, gousb.ErrorNoDevice)
}
