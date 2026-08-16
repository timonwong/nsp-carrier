package transport_test

import (
	"context"
	"testing"

	"github.com/timonwong/nsp-carrier/internal/transport"
)

func TestFullIOHandlesTimeoutsAndShortTransfers(t *testing.T) {
	link := transport.NewMemory(
		[]byte("request"),
		transport.WithReadFaults(transport.ErrTimeout),
		transport.WithWriteFaults(transport.ErrTimeout),
		transport.WithMaxRead(2), transport.WithMaxWrite(3),
	)
	read := make([]byte, len("request"))
	if err := transport.ReadFull(context.Background(), link, read); err != nil || string(read) != "request" {
		t.Fatalf("ReadFull() = %q, %v", read, err)
	}
	if err := transport.WriteFull(context.Background(), link, []byte("response")); err != nil || string(link.Written()) != "response" {
		t.Fatalf("WriteFull() = %q, %v", link.Written(), err)
	}
}
