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

func (c Connector) Open(context.Context) (host.Connection, error) {
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
