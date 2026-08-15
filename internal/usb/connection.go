package usb

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/gousb"
	"github.com/timonwong/ya-dbibackend/internal/transport"
)

const (
	VendorID  gousb.ID = 0x057e
	ProductID gousb.ID = 0x3000
)

var (
	ErrDeviceNotFound  = errors.New("DBI USB device not found")
	ErrMultipleDevices = errors.New("multiple DBI USB devices found")
)

type OpenOptions struct {
	ResetOnConnect bool
	Debug          int
}

type DeviceInfo struct {
	Vendor    gousb.ID
	Product   gousb.ID
	Bus       int
	Address   int
	Speed     gousb.Speed
	Selection EndpointSelection
}

type Connection struct {
	mu              sync.Mutex
	context         *gousb.Context
	device          *gousb.Device
	config          *gousb.Config
	interfaceHandle *gousb.Interface
	in              *gousb.InEndpoint
	out             *gousb.OutEndpoint
	info            DeviceInfo
	closed          bool
}

var _ transport.Duplex = (*Connection)(nil)

func Open(options OpenOptions) (*Connection, error) {
	usbContext := gousb.NewContext()
	if options.Debug > 0 {
		usbContext.Debug(options.Debug)
	}
	devices, enumerateErr := usbContext.OpenDevices(func(descriptor *gousb.DeviceDesc) bool {
		return descriptor.Vendor == VendorID && descriptor.Product == ProductID
	})
	if len(devices) == 0 {
		_ = usbContext.Close()
		if enumerateErr != nil {
			return nil, enumerateErr
		}
		return nil, ErrDeviceNotFound
	}
	if len(devices) != 1 || enumerateErr != nil {
		for _, device := range devices {
			_ = device.Close()
		}
		_ = usbContext.Close()
		if len(devices) != 1 {
			return nil, ErrMultipleDevices
		}
		return nil, enumerateErr
	}

	device := devices[0]
	fail := func(err error) (*Connection, error) {
		_ = device.Close()
		_ = usbContext.Close()
		return nil, err
	}
	if options.ResetOnConnect {
		if err := device.Reset(); err != nil {
			return fail(fmt.Errorf("reset DBI device: %w", err))
		}
		time.Sleep(time.Second)
	}

	selection, err := FindBulkPair(device.Desc)
	if err != nil {
		return fail(err)
	}
	config, err := device.Config(selection.Config)
	if err != nil {
		return fail(fmt.Errorf("claim USB configuration %d: %w", selection.Config, err))
	}
	failConfig := func(err error) (*Connection, error) {
		_ = config.Close()
		return fail(err)
	}
	interfaceHandle, err := config.Interface(selection.Interface, selection.Alternate)
	if err != nil {
		return failConfig(fmt.Errorf("claim USB interface %d alternate %d: %w", selection.Interface, selection.Alternate, err))
	}
	failInterface := func(err error) (*Connection, error) {
		interfaceHandle.Close()
		return failConfig(err)
	}
	in, err := interfaceHandle.InEndpoint(selection.InEndpoint)
	if err != nil {
		return failInterface(fmt.Errorf("open bulk IN endpoint %d: %w", selection.InEndpoint, err))
	}
	out, err := interfaceHandle.OutEndpoint(selection.OutEndpoint)
	if err != nil {
		return failInterface(fmt.Errorf("open bulk OUT endpoint %d: %w", selection.OutEndpoint, err))
	}

	return &Connection{
		context:         usbContext,
		device:          device,
		config:          config,
		interfaceHandle: interfaceHandle,
		in:              in,
		out:             out,
		info: DeviceInfo{
			Vendor:    device.Desc.Vendor,
			Product:   device.Desc.Product,
			Bus:       device.Desc.Bus,
			Address:   device.Desc.Address,
			Speed:     device.Desc.Speed,
			Selection: selection,
		},
	}, nil
}

func (c *Connection) Info() DeviceInfo { return c.info }

func (c *Connection) Read(ctx context.Context, destination []byte) (int, error) {
	read, err := c.in.ReadContext(ctx, destination)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return read, ctxErr
	}
	return read, err
}

func (c *Connection) Write(ctx context.Context, source []byte) (int, error) {
	written, err := c.out.WriteContext(ctx, source)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return written, ctxErr
	}
	return written, err
}

func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true

	if c.interfaceHandle != nil {
		c.interfaceHandle.Close()
		c.interfaceHandle = nil
	}
	var closeErr error
	if c.config != nil {
		closeErr = errors.Join(closeErr, c.config.Close())
		c.config = nil
	}
	if c.device != nil {
		closeErr = errors.Join(closeErr, c.device.Close())
		c.device = nil
	}
	if c.context != nil {
		closeErr = errors.Join(closeErr, c.context.Close())
		c.context = nil
	}
	return closeErr
}

// ForceReset releases claimed resources and resets the device. It is a
// last-resort fallback after the session context has been cancelled and a
// bounded graceful shutdown has failed.
func (c *Connection) ForceReset() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.device == nil {
		return nil
	}
	if c.interfaceHandle != nil {
		c.interfaceHandle.Close()
		c.interfaceHandle = nil
	}
	if c.config != nil {
		if err := c.config.Close(); err != nil {
			return err
		}
		c.config = nil
	}
	return c.device.Reset()
}
