package host_test

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/timonwong/nsp-carrier/internal/awoo"
	"github.com/timonwong/nsp-carrier/internal/dbi"
	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/goldleaf"
	"github.com/timonwong/nsp-carrier/internal/host"
	"github.com/timonwong/nsp-carrier/internal/protocoltrace"
	"github.com/timonwong/nsp-carrier/internal/transport"
)

func TestRunnerDispatchesDBIAndOwnsServingSessionState(t *testing.T) {
	catalog, err := files.BuildCatalog(nil, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	exit := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeRequest, Command: dbi.CommandExit})
	link := transport.NewMemory(exit[:])
	var events []host.Event

	err = host.NewRunner().Run(context.Background(), host.Request{
		Profile:   host.ProfileDBI,
		Catalog:   catalog,
		Connector: connector(link),
		Observe:   func(event host.Event) { events = append(events, event) },
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(events) < 4 || events[0].State != host.StateWaitingForDevice || events[1].State != host.StateConnected || events[2].State != host.StateServing || events[len(events)-1].State != host.StateCompleted {
		t.Fatalf("events = %#v", events)
	}
	if events[1].SessionID == "" || events[1].SessionID != events[len(events)-1].SessionID {
		t.Fatalf("session identity = %#v", events)
	}
	want := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeResponse, Command: dbi.CommandExit})
	if string(link.Written()) != string(want[:]) {
		t.Fatalf("wire response = %x, want %x", link.Written(), want)
	}
}

func TestRunnerDispatchesAwooThroughTheSharedSessionLifecycle(t *testing.T) {
	catalog, err := files.BuildCatalog(nil, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	exit := awoo.EncodeCommandHeader(awoo.CommandHeader{Type: awoo.CommandTypeRequest, Command: awoo.CommandExit})
	link := transport.NewMemory(exit[:])
	var terminal host.Event
	err = host.NewRunner().Run(context.Background(), host.Request{
		Profile: host.ProfileAwoo, Catalog: catalog, Connector: connector(link),
		Observe: func(event host.Event) { terminal = event },
	})
	if err != nil || terminal.State != host.StateCompleted || terminal.Profile != host.ProfileAwoo {
		t.Fatalf("Run() error = %v, terminal = %#v", err, terminal)
	}
	want := awoo.EncodeListHeader(0)
	if string(link.Written()) != string(want[:]) {
		t.Fatalf("Awoo opening handshake = %x, want %x", link.Written(), want)
	}
}

func TestRunnerExposesProtocolTraceOnlyWhenRequested(t *testing.T) {
	catalog, err := files.BuildCatalog(nil, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	exit := awoo.EncodeCommandHeader(awoo.CommandHeader{Type: awoo.CommandTypeRequest, Command: awoo.CommandExit})
	var terminal host.Event
	err = host.NewRunner().Run(context.Background(), host.Request{
		Profile: host.ProfileAwoo, Catalog: catalog, Connector: connector(transport.NewMemory(exit[:])),
		TraceProtocol: true,
		Observe:       func(event host.Event) { terminal = event },
	})
	if err != nil {
		t.Fatal(err)
	}
	if terminal.ProtocolTraceTruncated || len(terminal.ProtocolTrace) != 2 {
		t.Fatalf("terminal trace = %#v, truncated=%t", terminal.ProtocolTrace, terminal.ProtocolTraceTruncated)
	}
	if terminal.ProtocolTrace[0].Operation != "file_list" || terminal.ProtocolTrace[1].Operation != "exit" {
		t.Fatalf("terminal trace = %#v", terminal.ProtocolTrace)
	}

	terminal = host.Event{}
	err = host.NewRunner().Run(context.Background(), host.Request{
		Profile: host.ProfileAwoo, Catalog: catalog, Connector: connector(transport.NewMemory(exit[:])),
		Observe: func(event host.Event) { terminal = event },
	})
	if err != nil || len(terminal.ProtocolTrace) != 0 {
		t.Fatalf("disabled trace = %#v, error=%v", terminal.ProtocolTrace, err)
	}
}

func TestRunnerDispatchesGoldleafAndReportsReadOnlyWarnings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.nsp")
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	requestBlock := make([]byte, goldleaf.BlockSize)
	copy(requestBlock[:4], "GLCI")
	binary.LittleEndian.PutUint32(requestBlock[4:8], uint32(goldleaf.CommandDelete))
	binary.LittleEndian.PutUint32(requestBlock[8:12], uint32(len("VIRT:/game.nsp")))
	copy(requestBlock[12:], "VIRT:/game.nsp")
	request := make([]byte, 0, goldleaf.BlockSize*301)
	for range 301 {
		request = append(request, requestBlock...)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var disconnected host.Event
	err = host.NewRunner().Run(ctx, host.Request{
		Profile: host.ProfileGoldleaf, Catalog: catalog,
		Connector: &scriptedConnector{
			connections: []host.Connection{&memoryConnection{Memory: transport.NewMemory(request)}},
			errors:      []error{nil, host.ErrDeviceNotFound},
		},
		Observe: func(event host.Event) {
			if event.State == host.StateDisconnected {
				disconnected = event
				cancel()
			}
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
	if disconnected.Profile != host.ProfileGoldleaf || len(disconnected.Warnings) != 300 {
		t.Fatalf("disconnected event = %#v", disconnected)
	}
	warning := disconnected.Warnings[len(disconnected.Warnings)-1]
	if warning.Sequence != 301 || warning.Operation != "delete" || warning.Code != "read-only-virtual-catalog" {
		t.Fatalf("warning = %#v", warning)
	}
	if len(disconnected.ProtocolTrace) != 0 || disconnected.ProtocolTraceTruncated {
		t.Fatalf("trace should be disabled: %#v", disconnected.ProtocolTrace)
	}
}

func TestRunnerBoundsRequestedProtocolTrace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.nsp")
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	requestBlock := make([]byte, goldleaf.BlockSize)
	copy(requestBlock[:4], "GLCI")
	binary.LittleEndian.PutUint32(requestBlock[4:8], uint32(goldleaf.CommandGetDriveCount))
	request := make([]byte, 0, goldleaf.BlockSize*(protocoltrace.MaxRecords+1))
	for range protocoltrace.MaxRecords + 1 {
		request = append(request, requestBlock...)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var disconnected host.Event
	err = host.NewRunner().Run(ctx, host.Request{
		Profile: host.ProfileGoldleaf, Catalog: catalog, TraceProtocol: true,
		Connector: &scriptedConnector{
			connections: []host.Connection{&memoryConnection{Memory: transport.NewMemory(request)}},
			errors:      []error{nil, host.ErrDeviceNotFound},
		},
		Observe: func(event host.Event) {
			if event.State == host.StateDisconnected {
				disconnected = event
				cancel()
			}
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v", err)
	}
	if len(disconnected.ProtocolTrace) != protocoltrace.MaxRecords || !disconnected.ProtocolTraceTruncated {
		t.Fatalf("trace records=%d truncated=%t", len(disconnected.ProtocolTrace), disconnected.ProtocolTraceTruncated)
	}
}

func TestRunnerRejectsUnknownProfileBeforeWireIO(t *testing.T) {
	catalog, err := files.BuildCatalog(nil, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	err = host.NewRunner().Run(context.Background(), host.Request{
		Profile: "unknown", Catalog: catalog, Connector: connector(transport.NewMemory(nil)),
	})
	if !errors.Is(err, host.ErrUnknownProfile) {
		t.Fatalf("Run() error = %v, want ErrUnknownProfile", err)
	}
}

func TestRunnerValidatesDBIWireProjectionBeforeOpeningUSB(t *testing.T) {
	root := t.TempDir()
	paths := []string{
		filepath.Join(root, "left", "game.nsp"),
		filepath.Join(root, "right", "game.nsp"),
	}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := files.BuildCatalog(paths, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	connector := &scriptedConnector{}
	err = host.NewRunner().Run(context.Background(), host.Request{
		Profile: host.ProfileDBI, Catalog: catalog, Connector: connector,
	})
	if !errors.Is(err, files.ErrDuplicateBasename) || connector.index != 0 {
		t.Fatalf("Run() error = %v, connector opens = %d", err, connector.index)
	}
}

func TestRunnerClassifiesCancellationDisconnectAndProtocolFailure(t *testing.T) {
	catalog, err := files.BuildCatalog(nil, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		ctx       func() context.Context
		link      transport.Duplex
		wantErr   error
		wantState host.State
	}{
		{
			name: "cancellation",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			link: transport.NewMemory(nil), wantErr: context.Canceled, wantState: host.StateStopping,
		},
		{
			name: "malformed protocol frame", ctx: context.Background,
			link: transport.NewMemory(make([]byte, dbi.HeaderSize)), wantErr: dbi.ErrProtocol, wantState: host.StateFailed,
		},
		{
			name: "transfer cancellation", ctx: context.Background,
			link:    transport.NewMemory(nil, transport.WithReadFaults(context.Canceled)),
			wantErr: context.Canceled, wantState: host.StateStopping,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var terminal host.Event
			err := host.NewRunner().Run(test.ctx(), host.Request{
				Profile: host.ProfileDBI, Catalog: catalog, Connector: connector(test.link),
				Observe: func(event host.Event) { terminal = event },
			})
			if !errors.Is(err, test.wantErr) || terminal.State != test.wantState || !errors.Is(terminal.Err, test.wantErr) {
				t.Fatalf("Run() error = %v, terminal = %#v", err, terminal)
			}
		})
	}

	t.Run("disconnect returns to waiting for a fresh session", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var disconnected host.Event
		err := host.NewRunner().Run(ctx, host.Request{
			Profile: host.ProfileDBI, Catalog: catalog,
			Connector: &scriptedConnector{
				connections: []host.Connection{&memoryConnection{Memory: transport.NewMemory(nil)}},
				errors:      []error{nil, host.ErrDeviceNotFound},
			},
			Observe: func(event host.Event) {
				if event.State == host.StateDisconnected {
					disconnected = event
					cancel()
				}
			},
		})
		if !errors.Is(err, context.Canceled) || disconnected.State != host.StateDisconnected || !errors.Is(disconnected.Err, transport.ErrDisconnected) {
			t.Fatalf("Run() error = %v, disconnected = %#v", err, disconnected)
		}
	})
}

func TestRunnerCreatesFreshServingSessionIdentity(t *testing.T) {
	catalog, err := files.BuildCatalog(nil, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	var sessionIDs []string
	exit := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeRequest, Command: dbi.CommandExit})
	err = host.NewRunner().Run(context.Background(), host.Request{
		Profile: host.ProfileDBI, Catalog: catalog,
		Connector: &scriptedConnector{connections: []host.Connection{
			&memoryConnection{Memory: transport.NewMemory(nil)},
			&memoryConnection{Memory: transport.NewMemory(exit[:])},
		}},
		Observe: func(event host.Event) {
			if event.State == host.StateConnected {
				sessionIDs = append(sessionIDs, event.SessionID)
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sessionIDs) != 2 || sessionIDs[0] == "" || sessionIDs[0] == sessionIDs[1] {
		t.Fatalf("session IDs = %q", sessionIDs)
	}
}

type memoryConnection struct{ *transport.Memory }

func (c *memoryConnection) Close() error                   { return nil }
func (c *memoryConnection) Shutdown(context.Context) error { return nil }

type scriptedConnector struct {
	connections []host.Connection
	errors      []error
	index       int
}

func (c *scriptedConnector) Open(context.Context) (host.Connection, error) {
	index := c.index
	c.index++
	if index < len(c.errors) && c.errors[index] != nil {
		return nil, c.errors[index]
	}
	if index >= len(c.connections) {
		return nil, errors.New("scripted connector exhausted")
	}
	return c.connections[index], nil
}

func connector(link transport.Duplex) host.Connector {
	memory, ok := link.(*transport.Memory)
	if !ok {
		panic("test connector requires transport.Memory")
	}
	return &scriptedConnector{connections: []host.Connection{&memoryConnection{Memory: memory}}}
}

func TestRunnerReportsProgressByStableSourceID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.nsp")
	if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	detail := make([]byte, 16+len("game.nsp"))
	binary.LittleEndian.PutUint32(detail[0:4], 4)
	binary.LittleEndian.PutUint64(detail[4:12], 3)
	binary.LittleEndian.PutUint32(detail[12:16], uint32(len("game.nsp")))
	copy(detail[16:], "game.nsp")
	rangeRequest := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeRequest, Command: dbi.CommandFileRange, PayloadSize: uint32(len(detail))})
	rangeAck := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeAcknowledgement, Command: dbi.CommandFileRange})
	exit := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeRequest, Command: dbi.CommandExit})
	input := append(append(append(rangeRequest[:], detail...), rangeAck[:]...), exit[:]...)
	var terminal host.Event

	err = host.NewRunner().Run(context.Background(), host.Request{
		Profile:   host.ProfileDBI,
		Catalog:   catalog,
		Connector: connector(transport.NewMemory(input)),
		Observe:   func(event host.Event) { terminal = event },
	})
	if err != nil {
		t.Fatal(err)
	}
	sourceID := catalog.Entries()[0].ID
	progress := terminal.Progress[sourceID]
	if progress.SourceID != sourceID || progress.UniqueServedBytes != 4 || progress.WireBytes != 4 || progress.RangeRequests != 1 {
		t.Fatalf("terminal progress = %#v", terminal.Progress)
	}
}

func TestRunnerMarksCompletedAwooRequestedRangesFullyServed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.xci")
	if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	detail := make([]byte, awoo.RangeMetadataSize+len("game.xci"))
	binary.LittleEndian.PutUint64(detail[0:8], 4)
	binary.LittleEndian.PutUint64(detail[8:16], 3)
	binary.LittleEndian.PutUint64(detail[16:24], uint64(len("game.xci")))
	copy(detail[awoo.RangeMetadataSize:], "game.xci")
	rangeRequest := awoo.EncodeCommandHeader(awoo.CommandHeader{
		Type: awoo.CommandTypeRequest, Command: awoo.CommandFileRange, DataSize: uint64(len(detail)),
	})
	exit := awoo.EncodeCommandHeader(awoo.CommandHeader{Type: awoo.CommandTypeRequest, Command: awoo.CommandExit})
	input := append(append(rangeRequest[:], detail...), exit[:]...)
	var terminal host.Event

	err = host.NewRunner().Run(context.Background(), host.Request{
		Profile:   host.ProfileAwoo,
		Catalog:   catalog,
		Connector: connector(transport.NewMemory(input)),
		Observe:   func(event host.Event) { terminal = event },
	})
	if err != nil {
		t.Fatal(err)
	}
	sourceID := catalog.Entries()[0].ID
	progress := terminal.Progress[sourceID]
	if terminal.State != host.StateCompleted || progress.State != host.FileFullyServed ||
		progress.UniqueServedBytes != 4 || progress.WireBytes != 4 || progress.RangeRequests != 1 {
		t.Fatalf("terminal event = %#v", terminal)
	}
}
