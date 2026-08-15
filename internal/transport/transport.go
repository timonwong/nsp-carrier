package transport

import (
	"context"
	"errors"
)

var ErrTimeout = errors.New("transport timeout")

type Duplex interface {
	Read(context.Context, []byte) (int, error)
	Write(context.Context, []byte) (int, error)
}
