package usb

import (
	"context"
	"errors"
	"fmt"

	"github.com/timonwong/nsp-carrier/internal/host"
)

type Connector struct {
	Options OpenOptions
	OnOpen  func(DeviceInfo)
	open    func(OpenOptions) (*Connection, error)
}

func (c Connector) Open(ctx context.Context) (host.Connection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	open := c.open
	if open == nil {
		open = Open
	}
	connection, err := open(c.Options)
	if errors.Is(err, ErrDeviceNotFound) {
		return nil, host.ErrDeviceNotFound
	}
	if errors.Is(err, ErrDeviceUnavailable) {
		return nil, fmt.Errorf("%w: %w", host.ErrDeviceUnavailable, err)
	}
	if err != nil {
		return nil, err
	}
	if c.OnOpen != nil {
		c.OnOpen(connection.Info())
	}
	return connection, nil
}
