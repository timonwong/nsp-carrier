package transport

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryReportsEndOfStreamAsDisconnected(t *testing.T) {
	memory := NewMemory(nil)
	if _, err := memory.Read(context.Background(), make([]byte, 1)); !errors.Is(err, ErrDisconnected) {
		t.Fatalf("Read() error = %v, want ErrDisconnected", err)
	}
}
