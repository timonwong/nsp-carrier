package usb

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/gousb"
	"github.com/timonwong/nsp-carrier/internal/transport"
)

const (
	VendorID  gousb.ID = 0x057e
	ProductID gousb.ID = 0x3000
)

var (
	ErrDeviceNotFound  = errors.New("DBI USB device not found")
	ErrMultipleDevices = errors.New("multiple DBI USB devices found")
	ErrClosed          = errors.New("USB connection closed")
	ErrTransferActive  = errors.New("USB transfer still active")
	ErrShutdownTimeout = errors.New("USB shutdown timed out")
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

type inEndpoint interface {
	ReadContext(context.Context, []byte) (int, error)
}

type outEndpoint interface {
	WriteContext(context.Context, []byte) (int, error)
}

type connectionResources interface {
	Close() error
}

type gousbResources struct {
	context         *gousb.Context
	device          *gousb.Device
	config          *gousb.Config
	interfaceHandle *gousb.Interface
}

func (r *gousbResources) Close() error {
	if r.interfaceHandle != nil {
		r.interfaceHandle.Close()
		r.interfaceHandle = nil
	}
	var closeErr error
	if r.config != nil {
		closeErr = errors.Join(closeErr, r.config.Close())
		r.config = nil
	}
	if r.device != nil {
		closeErr = errors.Join(closeErr, r.device.Close())
		r.device = nil
	}
	if r.context != nil {
		closeErr = errors.Join(closeErr, r.context.Close())
		r.context = nil
	}
	return closeErr
}

type Connection struct {
	mu        sync.Mutex
	in        inEndpoint
	out       outEndpoint
	resources connectionResources
	info      DeviceInfo

	lifetimeCancel context.CancelFunc
	lifetime       context.Context
	active         int
	closing        bool
	drained        chan struct{}
	finalizeOnce   sync.Once
	closeDone      chan struct{}
	closeErr       error
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

	connection := newConnection(in, out, &gousbResources{
		context:         usbContext,
		device:          device,
		config:          config,
		interfaceHandle: interfaceHandle,
	})
	connection.info = DeviceInfo{
		Vendor:    device.Desc.Vendor,
		Product:   device.Desc.Product,
		Bus:       device.Desc.Bus,
		Address:   device.Desc.Address,
		Speed:     device.Desc.Speed,
		Selection: selection,
	}
	return connection, nil
}

func newConnection(in inEndpoint, out outEndpoint, resources connectionResources) *Connection {
	lifetime, cancel := context.WithCancel(context.Background())
	return &Connection{
		in:             in,
		out:            out,
		resources:      resources,
		lifetime:       lifetime,
		lifetimeCancel: cancel,
		drained:        make(chan struct{}),
		closeDone:      make(chan struct{}),
	}
}

func (c *Connection) Info() DeviceInfo { return c.info }

func (c *Connection) Read(ctx context.Context, destination []byte) (int, error) {
	endpoint, err := c.beginRead()
	if err != nil {
		return 0, err
	}
	defer c.endTransfer()
	opCtx, cancel := c.transferContext(ctx)
	defer cancel()

	read, err := endpoint.ReadContext(opCtx, destination)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return read, ctxErr
	}
	return read, normalizeTransferError(err)
}

func (c *Connection) Write(ctx context.Context, source []byte) (int, error) {
	endpoint, err := c.beginWrite()
	if err != nil {
		return 0, err
	}
	defer c.endTransfer()
	opCtx, cancel := c.transferContext(ctx)
	defer cancel()

	written, err := endpoint.WriteContext(opCtx, source)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return written, ctxErr
	}
	return written, normalizeTransferError(err)
}

func normalizeTransferError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gousb.TransferNoDevice),
		errors.Is(err, gousb.TransferError),
		errors.Is(err, gousb.ErrorNoDevice),
		errors.Is(err, gousb.ErrorIO),
		errors.Is(err, gousb.ErrorPipe),
		errors.Is(err, gousb.ErrorOther):
		return fmt.Errorf("%w: %v", transport.ErrDisconnected, err)
	case errors.Is(err, gousb.TransferTimedOut), errors.Is(err, gousb.ErrorTimeout):
		return fmt.Errorf("%w: %v", transport.ErrTimeout, err)
	default:
		// Keep TransferStall distinct: Windows may use it for ERROR_GEN_FAILURE,
		// but it also represents a recoverable endpoint stall.
		return err
	}
}

// Close starts connection shutdown without waiting for an active transfer.
// Resources are closed automatically when the final transfer drains.
func (c *Connection) Close() error {
	drained := c.startClosing()
	select {
	case <-drained:
		return c.finalizeClose()
	default:
		return ErrTransferActive
	}
}

// Shutdown cancels the connection-owned transfer context and waits until every
// in-flight transfer has returned before releasing the USB resources.
func (c *Connection) Shutdown(ctx context.Context) error {
	drained := c.startClosing()
	select {
	case <-drained:
		return c.finalizeClose()
	default:
	}
	select {
	case <-drained:
		return c.finalizeClose()
	case <-ctx.Done():
		return fmt.Errorf("%w: %w", ErrShutdownTimeout, ctx.Err())
	}
}

func (c *Connection) beginRead() (inEndpoint, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing || c.in == nil {
		return nil, ErrClosed
	}
	c.active++
	return c.in, nil
}

func (c *Connection) beginWrite() (outEndpoint, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing || c.out == nil {
		return nil, ErrClosed
	}
	c.active++
	return c.out, nil
}

func (c *Connection) transferContext(ctx context.Context) (context.Context, context.CancelFunc) {
	opCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(c.lifetime, cancel)
	return opCtx, func() {
		stop()
		cancel()
	}
}

func (c *Connection) endTransfer() {
	shouldFinalize := false
	c.mu.Lock()
	c.active--
	if c.closing && c.active == 0 {
		close(c.drained)
		shouldFinalize = true
	}
	c.mu.Unlock()
	if shouldFinalize {
		_ = c.finalizeClose()
	}
}

func (c *Connection) startClosing() <-chan struct{} {
	c.mu.Lock()
	first := !c.closing
	if first {
		c.closing = true
		c.in = nil
		c.out = nil
		if c.active == 0 {
			close(c.drained)
		}
	}
	drained := c.drained
	c.mu.Unlock()
	if first {
		c.lifetimeCancel()
	}
	return drained
}

func (c *Connection) finalizeClose() error {
	c.finalizeOnce.Do(func() {
		c.mu.Lock()
		resources := c.resources
		c.resources = nil
		c.mu.Unlock()

		var closeErr error
		if resources != nil {
			closeErr = resources.Close()
		}

		c.mu.Lock()
		c.closeErr = closeErr
		c.mu.Unlock()
		close(c.closeDone)
	})
	<-c.closeDone
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeErr
}
