package transport

import (
	"context"
	"errors"
	"io"
)

func ReadFull(ctx context.Context, link Duplex, destination []byte) error {
	for len(destination) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		read, err := link.Read(ctx, destination)
		if read > 0 {
			destination = destination[read:]
		}
		if errors.Is(err, ErrTimeout) {
			continue
		}
		if err != nil {
			return err
		}
		if read == 0 {
			return io.ErrNoProgress
		}
	}
	return nil
}

func WriteFull(ctx context.Context, link Duplex, source []byte) error {
	_, err := WriteFullCount(ctx, link, source)
	return err
}

func WriteFullCount(ctx context.Context, link Duplex, source []byte) (int, error) {
	total := 0
	for len(source) > 0 {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		written, err := link.Write(ctx, source)
		if written > 0 {
			total += written
			source = source[written:]
		}
		if errors.Is(err, ErrTimeout) {
			continue
		}
		if err != nil {
			return total, err
		}
		if written == 0 {
			return total, io.ErrNoProgress
		}
	}
	return total, nil
}
