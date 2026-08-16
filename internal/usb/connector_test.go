package usb

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/timonwong/nsp-carrier/internal/dbi"
	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/host"
	"github.com/timonwong/nsp-carrier/internal/transport"
)

func TestConnectorDoesNotEnumerateUSBForCancelledSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (Connector{}).Open(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Open() error = %v, want context.Canceled", err)
	}
}

func TestConnectorMapsUnavailableDeviceToRecoverableHostError(t *testing.T) {
	connector := Connector{
		open: func(OpenOptions) (*Connection, error) {
			return nil, recoverableOpenError(errors.New("claim diagnostics"))
		},
	}

	if _, err := connector.Open(context.Background()); !errors.Is(err, host.ErrDeviceUnavailable) || !strings.Contains(err.Error(), "claim diagnostics") {
		t.Fatalf("Open() error = %v, want host.ErrDeviceUnavailable", err)
	}
}

func TestIdentitylessClaimFailureRetriesIntoConnectedSession(t *testing.T) {
	catalog, err := files.BuildCatalog(nil, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	exit := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeRequest, Command: dbi.CommandExit})
	link := transport.NewMemory(exit[:])
	attempts := 0
	connector := Connector{
		open: func(OpenOptions) (*Connection, error) {
			attempts++
			if attempts == 1 {
				return nil, recoverableOpenError(errors.New("failed to claim interface 0: libusb error lost by formatting"))
			}
			endpoint := memoryEndpoint{Memory: link}
			return newConnection(endpoint, endpoint, &fakeResources{}), nil
		},
	}
	var events []host.Event

	err = host.NewRunner().Run(context.Background(), host.Request{
		Profile:   host.ProfileDBI,
		Catalog:   catalog,
		Connector: connector,
		Observe:   func(event host.Event) { events = append(events, event) },
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	connected := false
	for _, event := range events {
		if event.State == host.StateFailed {
			t.Fatalf("events contain Failed after recoverable claim error: %#v", events)
		}
		connected = connected || event.State == host.StateConnected
	}
	if attempts != 2 || !connected {
		t.Fatalf("attempts = %d, connected = %t, want second attempt Connected", attempts, connected)
	}
}

type memoryEndpoint struct {
	*transport.Memory
}

func (e memoryEndpoint) ReadContext(ctx context.Context, destination []byte) (int, error) {
	return e.Read(ctx, destination)
}

func (e memoryEndpoint) WriteContext(ctx context.Context, source []byte) (int, error) {
	return e.Write(ctx, source)
}
