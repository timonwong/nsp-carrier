package transport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
)

type MemoryOption func(*Memory)

func WithMaxRead(size int) MemoryOption {
	return func(memory *Memory) { memory.maxRead = size }
}

func WithMaxWrite(size int) MemoryOption {
	return func(memory *Memory) { memory.maxWrite = size }
}

func WithReadFaults(faults ...error) MemoryOption {
	return func(memory *Memory) { memory.readFaults = append([]error(nil), faults...) }
}

type Memory struct {
	mu         sync.Mutex
	input      *bytes.Reader
	output     bytes.Buffer
	maxRead    int
	maxWrite   int
	readFaults []error
}

func NewMemory(input []byte, options ...MemoryOption) *Memory {
	memory := &Memory{input: bytes.NewReader(append([]byte(nil), input...))}
	for _, option := range options {
		option(memory)
	}
	return memory
}

func (m *Memory) Read(ctx context.Context, destination []byte) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.readFaults) > 0 {
		fault := m.readFaults[0]
		m.readFaults = m.readFaults[1:]
		return 0, fault
	}
	if m.maxRead > 0 && len(destination) > m.maxRead {
		destination = destination[:m.maxRead]
	}
	read, err := m.input.Read(destination)
	if errors.Is(err, io.EOF) {
		return read, ErrDisconnected
	}
	return read, err
}

func (m *Memory) Write(ctx context.Context, source []byte) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.maxWrite > 0 && len(source) > m.maxWrite {
		source = source[:m.maxWrite]
	}
	return m.output.Write(source)
}

func (m *Memory) Written() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]byte(nil), m.output.Bytes()...)
}
