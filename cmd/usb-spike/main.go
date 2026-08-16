package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/timonwong/nsp-carrier/internal/app"
	"github.com/timonwong/nsp-carrier/internal/dbi"
	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/transport"
	usbtransport "github.com/timonwong/nsp-carrier/internal/usb"
)

const shutdownGracePeriod = 2 * time.Second

type options struct {
	timeout        time.Duration
	verbose        bool
	json           bool
	probe          bool
	resetOnConnect bool
}

type logger struct {
	json    bool
	verbose bool
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "usb-spike:", err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	flags := flag.NewFlagSet("usb-spike", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var config options
	flags.DurationVar(&config.timeout, "timeout", 0, "stop after this duration (0 waits indefinitely)")
	flags.BoolVar(&config.verbose, "verbose", false, "include descriptor and local path details")
	flags.BoolVar(&config.json, "json", false, "emit newline-delimited JSON logs")
	flags.BoolVar(&config.probe, "probe", false, "discover and claim DBI USB endpoints without serving files")
	flags.BoolVar(&config.resetOnConnect, "reset-on-connect", true, "reset the DBI USB device before claiming it")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() == 0 && !config.probe {
		return errors.New("provide at least one NSP/NSZ/XCI/XCZ file or directory")
	}

	log := logger{json: config.json, verbose: config.verbose}
	var catalog *files.Catalog
	if !config.probe {
		var err error
		catalog, err = files.BuildCatalog(flags.Args())
		if err != nil {
			return err
		}
		if len(catalog.Entries()) == 0 {
			return errors.New("catalog contains no supported files")
		}
	}
	log.event("info", "environment", map[string]any{
		"go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH,
		"gousb": "v1.1.3", "vid": "057e", "pid": "3000", "probe": config.probe,
	})
	if catalog != nil {
		for _, entry := range catalog.Entries() {
			fields := map[string]any{"id": entry.ID, "name": entry.Name, "size": entry.Size}
			if config.verbose {
				fields["path"] = entry.Path
			}
			log.event("info", "catalog_file", fields)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if config.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, config.timeout)
		defer cancel()
	}

	machine := app.NewStateMachine()
	if err := machine.Transition(app.StateWaitingForDevice, ""); err != nil {
		return err
	}
	log.event("info", "state", map[string]any{"state": app.StateWaitingForDevice})

	waitingLogged := false
	currentSessionID := ""
	for {
		connection, err := usbtransport.Open(usbtransport.OpenOptions{
			ResetOnConnect: config.resetOnConnect,
			Debug:          debugLevel(config.verbose),
		})
		if errors.Is(err, usbtransport.ErrDeviceNotFound) {
			if !waitingLogged {
				log.event("info", "waiting_for_device", nil)
				waitingLogged = true
			}
			select {
			case <-ctx.Done():
				return stopMachine(machine, currentSessionID, ctx.Err(), log)
			case <-time.After(time.Second):
				continue
			}
		}
		if err != nil {
			return err
		}
		waitingLogged = false

		sessionID := time.Now().UTC().Format("20060102T150405.000000000Z")
		currentSessionID = sessionID
		if err := machine.Transition(app.StateConnected, sessionID); err != nil {
			_ = connection.Close()
			return err
		}
		info := connection.Info()
		log.event("info", "device_connected", map[string]any{
			"session_id": sessionID,
			"bus":        info.Bus, "address": info.Address, "speed": info.Speed.String(),
			"config": info.Selection.Config, "interface": info.Selection.Interface,
			"alternate":   info.Selection.Alternate,
			"in_endpoint": info.Selection.InEndpoint, "out_endpoint": info.Selection.OutEndpoint,
		})
		if config.probe {
			closeErr := connection.Close()
			if err := machine.Transition(app.StateCompleted, sessionID); err != nil {
				return errors.Join(closeErr, err)
			}
			log.event("info", "probe_complete", map[string]any{"session_id": sessionID})
			return stopMachine(machine, sessionID, closeErr, log)
		}
		if err := machine.Transition(app.StateServing, sessionID); err != nil {
			_ = connection.Close()
			return err
		}
		log.event("info", "state", map[string]any{"session_id": sessionID, "state": app.StateServing})

		server := dbi.NewServer(catalog)
		serveErr := serveWithBoundedShutdown(ctx, server, connection, log)
		closeErr := connection.Close()
		if serveErr == nil {
			if err := machine.Transition(app.StateCompleted, sessionID); err != nil {
				return err
			}
			logProgress(server, catalog, sessionID, log)
			log.event("info", "session_completed", map[string]any{"session_id": sessionID})
			if closeErr != nil {
				return closeErr
			}
			return stopMachine(machine, sessionID, nil, log)
		}
		if ctx.Err() != nil {
			return stopMachine(machine, sessionID, ctx.Err(), log)
		}
		if isDisconnect(serveErr) {
			if err := machine.Transition(app.StateDisconnected, sessionID); err != nil {
				return err
			}
			logProgress(server, catalog, sessionID, log)
			log.event("warning", "device_disconnected", map[string]any{
				"session_id": sessionID, "error": serveErr.Error(),
			})
			continue
		}
		if err := machine.Transition(app.StateFailed, sessionID); err != nil {
			return err
		}
		logProgress(server, catalog, sessionID, log)
		return errors.Join(serveErr, closeErr)
	}
}

func serveWithBoundedShutdown(ctx context.Context, server *dbi.Server, connection *usbtransport.Connection, log logger) error {
	result := make(chan error, 1)
	go func() { result <- server.Serve(ctx, connection) }()

	select {
	case err := <-result:
		return err
	case <-ctx.Done():
	}
	grace := time.NewTimer(shutdownGracePeriod)
	defer grace.Stop()
	select {
	case err := <-result:
		return err
	case <-grace.C:
		log.event("warning", "graceful_shutdown_timeout", nil)
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()
	if err := connection.Shutdown(shutdownCtx); err != nil {
		return errors.Join(ctx.Err(), fmt.Errorf("shutdown USB connection: %w", err))
	}
	select {
	case err := <-result:
		return errors.Join(ctx.Err(), err)
	case <-shutdownCtx.Done():
		return errors.New("USB session did not stop after transfer shutdown")
	}
}

func stopMachine(machine *app.StateMachine, sessionID string, cause error, log logger) error {
	if err := machine.Transition(app.StateStopping, sessionID); err != nil {
		return errors.Join(cause, err)
	}
	if err := machine.Transition(app.StateIdle, sessionID); err != nil {
		return errors.Join(cause, err)
	}
	log.event("info", "state", map[string]any{"state": app.StateIdle})
	if errors.Is(cause, context.Canceled) {
		return nil
	}
	return cause
}

func logProgress(server *dbi.Server, catalog *files.Catalog, sessionID string, log logger) {
	for _, entry := range catalog.Entries() {
		progress, ok := server.Progress(entry.Name)
		if !ok {
			continue
		}
		status := "NotRequested"
		if progress.UniqueServedBytes == progress.TotalBytes && progress.TotalBytes > 0 {
			status = "FullyServed"
		} else if progress.WireBytes > 0 {
			status = "Interrupted"
		}
		log.event("info", "file_progress", map[string]any{
			"session_id": sessionID, "name": entry.Name, "status": status,
			"unique_served_bytes": progress.UniqueServedBytes,
			"wire_bytes":          progress.WireBytes, "total_bytes": progress.TotalBytes,
			"range_requests":          progress.RangeRequests,
			"non_sequential_requests": progress.NonSequentialRequests,
			"backward_requests":       progress.BackwardRequests,
			"repeated_requests":       progress.RepeatedRequests,
			"max_requested_offset":    progress.MaxRequestedOffset,
		})
	}
}

func isDisconnect(err error) bool {
	return errors.Is(err, transport.ErrDisconnected)
}

func debugLevel(verbose bool) int {
	if verbose {
		return 3
	}
	return 0
}

func (l logger) event(level, message string, fields map[string]any) {
	if l.json {
		record := map[string]any{
			"time":  time.Now().UTC().Format(time.RFC3339Nano),
			"level": level, "event": message,
		}
		for key, value := range fields {
			record[key] = value
		}
		encoded, err := json.Marshal(record)
		if err != nil {
			fmt.Fprintln(os.Stderr, "log encoding error:", err)
			return
		}
		fmt.Println(string(encoded))
		return
	}
	fmt.Printf("[%s] %-7s %s", time.Now().Format("15:04:05"), level, message)
	for key, value := range fields {
		fmt.Printf(" %s=%v", key, value)
	}
	fmt.Println()
}
