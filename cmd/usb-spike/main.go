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

	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/host"
	usbtransport "github.com/timonwong/nsp-carrier/internal/usb"
)

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

	if config.probe {
		connection, err := usbtransport.Open(usbtransport.OpenOptions{
			ResetOnConnect: config.resetOnConnect,
			Debug:          debugLevel(config.verbose),
		})
		if err != nil {
			return err
		}
		info := connection.Info()
		closeErr := connection.Close()
		log.event("info", "probe_complete", map[string]any{"bus": info.Bus, "address": info.Address})
		return closeErr
	}

	var info usbtransport.DeviceInfo
	connector := usbtransport.Connector{
		Options: usbtransport.OpenOptions{
			ResetOnConnect: config.resetOnConnect,
			Debug:          debugLevel(config.verbose),
		},
		OnOpen: func(opened usbtransport.DeviceInfo) { info = opened },
	}
	err := host.NewRunner().Run(ctx, host.Request{
		Profile: host.ProfileDBI, Catalog: catalog, Connector: connector,
		Observe: func(event host.Event) {
			fields := map[string]any{
				"session_id": event.SessionID, "state": event.State, "profile": event.Profile,
			}
			if event.Err != nil {
				fields["error"] = event.Err.Error()
			}
			log.event("info", "state", fields)
			if event.State == host.StateConnected {
				log.event("info", "device_connected", map[string]any{
					"session_id": event.SessionID, "profile": event.Profile,
					"bus": info.Bus, "address": info.Address, "speed": info.Speed.String(),
					"config": info.Selection.Config, "interface": info.Selection.Interface,
					"alternate":   info.Selection.Alternate,
					"in_endpoint": info.Selection.InEndpoint, "out_endpoint": info.Selection.OutEndpoint,
				})
			}
			if event.State == host.StateCompleted || event.State == host.StateDisconnected || event.State == host.StateFailed || event.State == host.StateStopping {
				logProgress(event, catalog, log)
			}
		},
	})
	log.event("info", "state", map[string]any{"state": "Idle", "profile": host.ProfileDBI})
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func logProgress(event host.Event, catalog *files.Catalog, log logger) {
	for _, entry := range catalog.Entries() {
		progress, ok := event.Progress[entry.ID]
		if !ok {
			continue
		}
		log.event("info", "file_progress", map[string]any{
			"session_id": event.SessionID, "profile": event.Profile,
			"source_id": entry.ID, "name": entry.Name, "status": progress.State,
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
