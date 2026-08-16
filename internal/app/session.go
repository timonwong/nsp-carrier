package app

import (
	"context"

	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/host"
	usbtransport "github.com/timonwong/nsp-carrier/internal/usb"
)

func runUSBSession(ctx context.Context, catalog *files.Catalog, update func(sessionUpdate)) error {
	return host.NewRunner().Run(ctx, host.Request{
		Profile:   host.ProfileDBI,
		Catalog:   catalog,
		Connector: usbtransport.Connector{Options: usbtransport.OpenOptions{ResetOnConnect: false}},
		Observe: func(event host.Event) {
			var eventErr error
			if event.State == host.StateFailed {
				eventErr = event.Err
			}
			update(sessionUpdate{
				state: State(event.State), sessionID: event.SessionID,
				progress: event.Progress, err: eventErr,
			})
		},
	})
}
