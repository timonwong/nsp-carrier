package app

import (
	"context"

	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/host"
	usbtransport "github.com/timonwong/nsp-carrier/internal/usb"
)

func runUSBSession(ctx context.Context, profile host.ProfileID, catalog *files.Catalog, update func(host.Event)) error {
	return host.NewRunner().Run(ctx, host.Request{
		Profile:   profile,
		Catalog:   catalog,
		Connector: usbtransport.Connector{Options: usbtransport.OpenOptions{ResetOnConnect: false}},
		Observe:   update,
	})
}
