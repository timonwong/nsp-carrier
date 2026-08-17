package host_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/timonwong/nsp-carrier/internal/awoo"
	"github.com/timonwong/nsp-carrier/internal/dbi"
	"github.com/timonwong/nsp-carrier/internal/files"
	"github.com/timonwong/nsp-carrier/internal/goldleaf"
	"github.com/timonwong/nsp-carrier/internal/host"
	"github.com/timonwong/nsp-carrier/internal/protocoltrace"
	"github.com/timonwong/nsp-carrier/internal/sphaira"
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

func TestRunnerServesCompleteSphairaSessionAtTheHostBoundary(t *testing.T) {
	root := t.TempDir()
	paths := []string{filepath.Join(root, "first.nsp"), filepath.Join(root, "second.msp")}
	for index, path := range paths {
		if err := os.WriteFile(path, []byte(fmt.Sprintf("file-%d-data", index)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := files.BuildCatalog(paths, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	handshake := sphaira.EncodeCommand(sphaira.CommandQuit, 0, 0)
	open := sphaira.EncodeCommand(sphaira.CommandOpen, 1, 0)
	rangePacket := sphaira.EncodeData(5, 10, 0)
	closePacket := sphaira.EncodeData(0, 0, 0)
	quit := sphaira.EncodeCommand(sphaira.CommandQuit, 0, 0)
	input := append(append(append(append(handshake[:], open[:]...), rangePacket[:]...), closePacket[:]...), quit[:]...)
	link := transport.NewMemory(input, transport.WithMaxRead(7), transport.WithMaxWrite(5), transport.WithReadFaults(transport.ErrTimeout))
	var terminal host.Event
	err = host.NewRunner().Run(context.Background(), host.Request{
		Profile: host.ProfileSphaira, Catalog: catalog, Connector: connector(link), TraceProtocol: true,
		Observe: func(event host.Event) { terminal = event },
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	list := []byte("first.nsp\nsecond.msp\n")
	want := append([]byte{}, sphairaPacket(sphaira.EncodeResult(sphaira.ResultOK, uint32(len(list)), 0))...)
	want = append(want, list...)
	want = append(want, sphairaPacket(sphaira.EncodeResult(sphaira.ResultOK, 0, uint32(len("file-1-data"))))...)
	payload := []byte("1-data")
	want = append(want, sphairaPacket(sphaira.EncodeResult(sphaira.ResultOK, uint32(len(payload)), sphaira.PayloadCRC32C(payload)))...)
	want = append(want, payload...)
	want = append(want, sphairaPacket(sphaira.EncodeResult(sphaira.ResultOK, 0, 0))...)
	want = append(want, sphairaPacket(sphaira.EncodeResult(sphaira.ResultOK, 0, 0))...)
	if got := link.Written(); string(got) != string(want) {
		t.Fatalf("wire output = %x\nwant = %x", got, want)
	}
	entry := catalog.Entries()[1]
	progress := terminal.Progress[entry.ID]
	if terminal.State != host.StateCompleted || progress.State != host.FileFullyServed ||
		progress.UniqueServedBytes != 6 || progress.RangeRequests != 1 {
		t.Fatalf("terminal event = %#v", terminal)
	}
	if len(terminal.ProtocolTrace) == 0 {
		t.Fatal("Sphaira session emitted no protocol trace")
	}
	var sawRangeIntegrity bool
	for _, record := range terminal.ProtocolTrace {
		if record.Operation == "range" && record.IntegrityChecked && record.IntegrityValid && record.Offset == 5 && record.Size == 10 {
			sawRangeIntegrity = true
		}
	}
	if !sawRangeIntegrity {
		t.Fatalf("trace = %#v", terminal.ProtocolTrace)
	}
}

func sphairaPacket(packet [sphaira.PacketSize]byte) []byte { return packet[:] }

func TestRunnerTracksWholeSourceCoverageForSphaira(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.nsp")
	if err := os.WriteFile(path, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	packets := [][sphaira.PacketSize]byte{
		sphaira.EncodeCommand(sphaira.CommandQuit, 0, 0),
		sphaira.EncodeCommand(sphaira.CommandOpen, 0, 0),
		sphaira.EncodeData(5, 5, 0),
		sphaira.EncodeData(0, 5, 0),
		sphaira.EncodeData(0, 5, 0),
		sphaira.EncodeData(8, 5, 0),
		sphaira.EncodeData(0, 0, 0),
		sphaira.EncodeCommand(sphaira.CommandQuit, 0, 0),
	}
	var input []byte
	for _, packet := range packets {
		input = append(input, packet[:]...)
	}
	var terminal host.Event
	if err := host.NewRunner().Run(context.Background(), host.Request{
		Profile: host.ProfileSphaira, Catalog: catalog, Connector: connector(transport.NewMemory(input)),
		Observe: func(event host.Event) { terminal = event },
	}); err != nil {
		t.Fatal(err)
	}
	progress := terminal.Progress[catalog.Entries()[0].ID]
	if progress.State != host.FileFullyServed || progress.UniqueServedBytes != 10 || progress.WireBytes != 17 ||
		progress.RangeRequests != 4 || progress.NonSequentialRequests != 3 || progress.BackwardRequests != 1 || progress.RepeatedRequests != 1 {
		t.Fatalf("progress = %#v", progress)
	}
}

func TestRunnerReconnectsSphairaWithFreshSessionIdentity(t *testing.T) {
	catalog, err := files.BuildCatalog(nil, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	handshake := sphaira.EncodeCommand(sphaira.CommandQuit, 0, 0)
	quit := sphaira.EncodeCommand(sphaira.CommandQuit, 0, 0)
	completedInput := append(handshake[:], quit[:]...)
	var sessions []string
	var sawDisconnected bool
	err = host.NewRunner().Run(context.Background(), host.Request{
		Profile: host.ProfileSphaira, Catalog: catalog,
		Connector: &scriptedConnector{connections: []host.Connection{
			&memoryConnection{Memory: transport.NewMemory(nil)},
			&memoryConnection{Memory: transport.NewMemory(completedInput)},
		}},
		Observe: func(event host.Event) {
			if event.State == host.StateConnected {
				sessions = append(sessions, event.SessionID)
			}
			if event.State == host.StateDisconnected {
				sawDisconnected = true
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sawDisconnected || len(sessions) != 2 || sessions[0] == sessions[1] {
		t.Fatalf("disconnected=%t sessions=%q", sawDisconnected, sessions)
	}
}

func TestRunnerServesSphairaOffsetAboveFourGiB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.xci")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	const offset = int64(1<<32 + 7)
	if err := file.Truncate(offset + 4); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte("tail"), offset); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	packets := [][sphaira.PacketSize]byte{
		sphaira.EncodeCommand(sphaira.CommandQuit, 0, 0),
		sphaira.EncodeCommand(sphaira.CommandOpen, 0, 0),
		sphaira.EncodeData(uint64(offset), 4, 0),
		sphaira.EncodeData(0, 0, 0),
		sphaira.EncodeCommand(sphaira.CommandQuit, 0, 0),
	}
	var input []byte
	for _, packet := range packets {
		input = append(input, packet[:]...)
	}
	link := transport.NewMemory(input)
	var terminal host.Event
	if err := host.NewRunner().Run(context.Background(), host.Request{
		Profile: host.ProfileSphaira, Catalog: catalog, Connector: connector(link),
		Observe: func(event host.Event) { terminal = event },
	}); err != nil {
		t.Fatal(err)
	}
	response := sphaira.EncodeResult(sphaira.ResultOK, 4, sphaira.PayloadCRC32C([]byte("tail")))
	if !bytes.Contains(link.Written(), append(response[:], []byte("tail")...)) {
		t.Fatalf("wire output = %x", link.Written())
	}
	progress := terminal.Progress[catalog.Entries()[0].ID]
	if progress.MaxRequestedOffset != uint64(offset) || progress.UniqueServedBytes != 4 {
		t.Fatalf("progress = %#v", progress)
	}
}

func TestRunnerClassifiesSphairaProtocolFailuresAtThePrimarySeam(t *testing.T) {
	t.Run("malformed packet", func(t *testing.T) {
		catalog, err := files.BuildCatalog(nil, host.AllSupportedExtensions())
		if err != nil {
			t.Fatal(err)
		}
		malformed := sphaira.EncodeCommand(sphaira.CommandQuit, 0, 0)
		malformed[20] ^= 0xff
		var terminal host.Event
		err = host.NewRunner().Run(context.Background(), host.Request{
			Profile: host.ProfileSphaira, Catalog: catalog, Connector: connector(transport.NewMemory(malformed[:])), TraceProtocol: true,
			Observe: func(event host.Event) { terminal = event },
		})
		if !errors.Is(err, sphaira.ErrProtocol) || terminal.State != host.StateFailed ||
			len(terminal.ProtocolTrace) != 1 || terminal.ProtocolTrace[0].IntegrityValid {
			t.Fatalf("error=%v terminal=%#v", err, terminal)
		}
	})

	t.Run("invalid index", func(t *testing.T) {
		catalog, err := files.BuildCatalog(nil, host.AllSupportedExtensions())
		if err != nil {
			t.Fatal(err)
		}
		handshake := sphaira.EncodeCommand(sphaira.CommandQuit, 0, 0)
		open := sphaira.EncodeCommand(sphaira.CommandOpen, 1, 0)
		input := append(handshake[:], open[:]...)
		var terminal host.Event
		err = host.NewRunner().Run(context.Background(), host.Request{
			Profile: host.ProfileSphaira, Catalog: catalog, Connector: connector(transport.NewMemory(input)),
			Observe: func(event host.Event) { terminal = event },
		})
		if !errors.Is(err, sphaira.ErrInvalidRequest) || terminal.State != host.StateFailed {
			t.Fatalf("error=%v terminal=%#v", err, terminal)
		}
	})

	t.Run("unknown command", func(t *testing.T) {
		catalog, err := files.BuildCatalog(nil, host.AllSupportedExtensions())
		if err != nil {
			t.Fatal(err)
		}
		handshake := sphaira.EncodeCommand(sphaira.CommandQuit, 0, 0)
		unknown := sphaira.EncodeCommand(99, 0, 0)
		input := append(handshake[:], unknown[:]...)
		var terminal host.Event
		err = host.NewRunner().Run(context.Background(), host.Request{
			Profile: host.ProfileSphaira, Catalog: catalog, Connector: connector(transport.NewMemory(input)),
			Observe: func(event host.Event) { terminal = event },
		})
		if !errors.Is(err, sphaira.ErrUnsupportedCommand) || terminal.State != host.StateFailed {
			t.Fatalf("error=%v terminal=%#v", err, terminal)
		}
	})

	t.Run("invalid range", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "game.nsp")
		if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
		catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
		if err != nil {
			t.Fatal(err)
		}
		handshake := sphaira.EncodeCommand(sphaira.CommandQuit, 0, 0)
		open := sphaira.EncodeCommand(sphaira.CommandOpen, 0, 0)
		invalid := sphaira.EncodeData(0, sphaira.MaxRangeSize+1, 0)
		input := append(append(handshake[:], open[:]...), invalid[:]...)
		var terminal host.Event
		err = host.NewRunner().Run(context.Background(), host.Request{
			Profile: host.ProfileSphaira, Catalog: catalog, Connector: connector(transport.NewMemory(input)),
			Observe: func(event host.Event) { terminal = event },
		})
		if !errors.Is(err, sphaira.ErrInvalidRequest) || terminal.State != host.StateFailed {
			t.Fatalf("error=%v terminal=%#v", err, terminal)
		}
	})

	t.Run("source mutation", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "game.nsp")
		if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
		catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("changed"), 0o644); err != nil {
			t.Fatal(err)
		}
		handshake := sphaira.EncodeCommand(sphaira.CommandQuit, 0, 0)
		open := sphaira.EncodeCommand(sphaira.CommandOpen, 0, 0)
		rangePacket := sphaira.EncodeData(0, 1, 0)
		input := append(append(handshake[:], open[:]...), rangePacket[:]...)
		var terminal host.Event
		err = host.NewRunner().Run(context.Background(), host.Request{
			Profile: host.ProfileSphaira, Catalog: catalog, Connector: connector(transport.NewMemory(input)),
			Observe: func(event host.Event) { terminal = event },
		})
		if !errors.Is(err, files.ErrSourceChanged) || terminal.State != host.StateFailed {
			t.Fatalf("error=%v terminal=%#v", err, terminal)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		catalog, err := files.BuildCatalog(nil, host.AllSupportedExtensions())
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		var terminal host.Event
		err = host.NewRunner().Run(ctx, host.Request{
			Profile: host.ProfileSphaira, Catalog: catalog, Connector: connector(transport.NewMemory(nil)),
			Observe: func(event host.Event) { terminal = event },
		})
		if !errors.Is(err, context.Canceled) || terminal.State != host.StateStopping {
			t.Fatalf("error=%v terminal=%#v", err, terminal)
		}
	})
}

func TestRunnerPreservesPartialSphairaWritesInFailedProgress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "game.nsp")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog, err := files.BuildCatalog([]string{path}, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	handshake := sphaira.EncodeCommand(sphaira.CommandQuit, 0, 0)
	open := sphaira.EncodeCommand(sphaira.CommandOpen, 0, 0)
	rangePacket := sphaira.EncodeData(0, 4, 0)
	input := append(append(handshake[:], open[:]...), rangePacket[:]...)
	writeErr := errors.New("write interrupted")
	connection := &limitedConnection{
		Memory: transport.NewMemory(input), remaining: 24 + len("game.nsp\n") + 24 + 24 + 2, err: writeErr,
	}
	var terminal host.Event
	err = host.NewRunner().Run(context.Background(), host.Request{
		Profile: host.ProfileSphaira, Catalog: catalog,
		Connector: &scriptedConnector{connections: []host.Connection{connection}},
		Observe:   func(event host.Event) { terminal = event },
	})
	progress := terminal.Progress[catalog.Entries()[0].ID]
	if !errors.Is(err, writeErr) || terminal.State != host.StateFailed || progress.State != host.FileFailed ||
		progress.UniqueServedBytes != 2 || progress.WireBytes != 2 {
		t.Fatalf("error=%v terminal=%#v", err, terminal)
	}
}

func TestRunnerRetriesRecoverableDeviceOpenFailure(t *testing.T) {
	catalog, err := files.BuildCatalog(nil, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	exit := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeRequest, Command: dbi.CommandExit})
	var events []host.Event

	err = host.NewRunner().Run(context.Background(), host.Request{
		Profile: host.ProfileDBI,
		Catalog: catalog,
		Connector: &scriptedConnector{
			connections: []host.Connection{nil, &memoryConnection{Memory: transport.NewMemory(exit[:])}},
			errors:      []error{host.ErrDeviceUnavailable},
		},
		Observe: func(event host.Event) { events = append(events, event) },
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, event := range events {
		if event.State == host.StateFailed {
			t.Fatalf("events contain Failed after recoverable open error: %#v", events)
		}
	}
	if events[len(events)-1].State != host.StateCompleted {
		t.Fatalf("terminal event = %#v, want Completed", events[len(events)-1])
	}
}

func TestRunnerFailsAfterRepeatedRecoverableDeviceOpenFailures(t *testing.T) {
	catalog, err := files.BuildCatalog(nil, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	connector := &failingConnector{err: host.ErrDeviceUnavailable}
	var terminal host.Event

	err = host.NewRunner().Run(context.Background(), host.Request{
		Profile:   host.ProfileDBI,
		Catalog:   catalog,
		Connector: connector,
		Observe:   func(event host.Event) { terminal = event },
	})
	if !errors.Is(err, host.ErrDeviceUnavailable) {
		t.Fatalf("Run() error = %v, want ErrDeviceUnavailable", err)
	}
	if connector.opens <= 1 || terminal.State != host.StateFailed {
		t.Fatalf("opens = %d, terminal = %#v, want retries then Failed", connector.opens, terminal)
	}
}

func TestRunnerResetsRecoverableOpenFailureBudgetAfterSuccessfulConnection(t *testing.T) {
	catalog, err := files.BuildCatalog(nil, host.AllSupportedExtensions())
	if err != nil {
		t.Fatal(err)
	}
	exit := dbi.EncodeHeader(dbi.Header{Type: dbi.CommandTypeRequest, Command: dbi.CommandExit})
	connector := &scriptedConnector{
		connections: []host.Connection{
			nil,
			&memoryConnection{Memory: transport.NewMemory(nil)},
			nil,
			nil,
			&memoryConnection{Memory: transport.NewMemory(exit[:])},
		},
		errors: []error{
			host.ErrDeviceUnavailable,
			nil,
			host.ErrDeviceUnavailable,
			host.ErrDeviceUnavailable,
			nil,
		},
	}
	var terminal host.Event

	err = host.NewRunner().Run(context.Background(), host.Request{
		Profile:   host.ProfileDBI,
		Catalog:   catalog,
		Connector: connector,
		Observe:   func(event host.Event) { terminal = event },
	})
	if err != nil || terminal.State != host.StateCompleted {
		t.Fatalf("Run() error = %v, terminal = %#v, want Completed after retry budget reset", err, terminal)
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

type limitedConnection struct {
	*transport.Memory
	remaining int
	err       error
}

func (c *limitedConnection) Write(ctx context.Context, source []byte) (int, error) {
	if c.remaining <= 0 {
		return 0, c.err
	}
	if len(source) > c.remaining {
		written, _ := c.Memory.Write(ctx, source[:c.remaining])
		c.remaining = 0
		return written, c.err
	}
	c.remaining -= len(source)
	return c.Memory.Write(ctx, source)
}

func (c *limitedConnection) Close() error                   { return nil }
func (c *limitedConnection) Shutdown(context.Context) error { return nil }

type scriptedConnector struct {
	connections []host.Connection
	errors      []error
	index       int
}

type failingConnector struct {
	err   error
	opens int
}

func (c *failingConnector) Open(context.Context) (host.Connection, error) {
	c.opens++
	return nil, c.err
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
