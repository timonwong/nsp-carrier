package usb

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/gousb"
	"github.com/timonwong/nsp-carrier/internal/transport"
)

func TestNormalizeTransferError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "transfer no device", err: gousb.TransferNoDevice, want: transport.ErrDisconnected},
		{name: "submit no device", err: gousb.ErrorNoDevice, want: transport.ErrDisconnected},
		{name: "transfer timeout", err: gousb.TransferTimedOut, want: transport.ErrTimeout},
		{name: "submit timeout", err: gousb.ErrorTimeout, want: transport.ErrTimeout},
		{name: "stall remains distinct", err: gousb.TransferStall, want: gousb.TransferStall},
		{name: "generic transfer error remains distinct", err: gousb.TransferError, want: gousb.TransferError},
		{name: "pipe remains distinct", err: gousb.ErrorPipe, want: gousb.ErrorPipe},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := normalizeTransferError(test.err); !errors.Is(err, test.want) {
				t.Fatalf("normalizeTransferError(%v) = %v, want errors.Is(_, %v)", test.err, err, test.want)
			}
		})
	}
}

func TestShutdownWaitsForInflightTransferBeforeClosingResources(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	endpoint := &blockingInEndpoint{started: started, release: release}
	resources := &fakeResources{}
	connection := newConnection(endpoint, nil, resources)

	readDone := make(chan error, 1)
	go func() {
		_, err := connection.Read(context.Background(), make([]byte, 1))
		readDone <- err
	}()
	<-started

	if err := connection.Close(); !errors.Is(err, ErrTransferActive) {
		t.Fatalf("Close() error = %v, want ErrTransferActive", err)
	}
	if got := resources.closeCalls.Load(); got != 0 {
		t.Fatalf("resources closed %d time(s) by Close while transfer was in flight", got)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := connection.Shutdown(shutdownCtx); !errors.Is(err, ErrShutdownTimeout) {
		t.Fatalf("Shutdown() error = %v, want ErrShutdownTimeout", err)
	}
	if got := resources.closeCalls.Load(); got != 0 {
		t.Fatalf("resources closed %d time(s) while transfer was in flight", got)
	}

	close(release)
	if err := <-readDone; err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got := resources.closeCalls.Load(); got != 1 {
		t.Fatalf("resources close calls = %d, want 1 after transfer drained", got)
	}
	if _, err := connection.Read(context.Background(), make([]byte, 1)); !errors.Is(err, ErrClosed) {
		t.Fatalf("Read() after shutdown error = %v, want ErrClosed", err)
	}
}

func TestShutdownCancelsConnectionOwnedTransferContext(t *testing.T) {
	started := make(chan struct{})
	endpoint := &cancellableInEndpoint{started: started}
	resources := &fakeResources{}
	connection := newConnection(endpoint, nil, resources)

	readDone := make(chan error, 1)
	go func() {
		_, err := connection.Read(context.Background(), make([]byte, 1))
		readDone <- err
	}()
	<-started

	if err := connection.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if err := <-readDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("Read() error = %v, want context.Canceled", err)
	}
	if got := resources.closeCalls.Load(); got != 1 {
		t.Fatalf("resources close calls = %d, want 1", got)
	}
}

type blockingInEndpoint struct {
	started chan struct{}
	release chan struct{}
}

func (e *blockingInEndpoint) ReadContext(context.Context, []byte) (int, error) {
	close(e.started)
	<-e.release
	return 0, nil
}

type cancellableInEndpoint struct {
	started chan struct{}
}

func (e *cancellableInEndpoint) ReadContext(ctx context.Context, _ []byte) (int, error) {
	close(e.started)
	<-ctx.Done()
	return 0, ctx.Err()
}

type fakeResources struct {
	closeCalls atomic.Int32
}

func (r *fakeResources) Close() error {
	r.closeCalls.Add(1)
	return nil
}
