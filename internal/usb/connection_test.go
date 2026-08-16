package usb

import (
	"errors"
	"testing"

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
