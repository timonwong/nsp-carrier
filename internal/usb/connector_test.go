package usb_test

import (
	"context"
	"errors"
	"testing"

	"github.com/timonwong/nsp-carrier/internal/usb"
)

func TestConnectorDoesNotEnumerateUSBForCancelledSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (usb.Connector{}).Open(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Open() error = %v, want context.Canceled", err)
	}
}
