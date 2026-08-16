package usb

import (
	"context"
	"errors"

	"github.com/timonwong/nsp-carrier/internal/host"
)

type Connector struct {
	Options OpenOptions
	OnOpen  func(DeviceInfo)
}

func (c Connector) Open(ctx context.Context) (host.Connection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	connection, err := Open(c.Options)
	if errors.Is(err, ErrDeviceNotFound) {
		return nil, host.ErrDeviceNotFound
	}
	if err != nil {
		return nil, err
	}
	if c.OnOpen != nil {
		c.OnOpen(connection.Info())
	}
	return connection, nil
}
