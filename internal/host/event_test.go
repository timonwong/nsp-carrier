package host_test

import (
	"errors"
	"testing"

	"github.com/timonwong/nsp-carrier/internal/host"
)

func TestEventSameStateIgnoresSnapshotPayload(t *testing.T) {
	baseline := host.Event{
		Profile:   host.ProfileGoldleaf,
		SessionID: "session-1",
		State:     host.StateServing,
		Progress: map[string]host.ProgressSnapshot{
			"source-1": {UniqueServedBytes: 1},
		},
	}

	tests := []struct {
		name  string
		event host.Event
		want  bool
	}{
		{
			name: "progress and warnings are snapshots, not state changes",
			event: host.Event{
				Profile:   host.ProfileGoldleaf,
				SessionID: "session-1",
				State:     host.StateServing,
				Progress: map[string]host.ProgressSnapshot{
					"source-1": {UniqueServedBytes: 2},
				},
				Warnings: []host.Warning{{Sequence: 1, Code: "read_only"}},
			},
			want: true,
		},
		{
			name: "new session",
			event: host.Event{Profile: host.ProfileGoldleaf, SessionID: "session-2",
				State: host.StateServing},
		},
		{
			name: "new state",
			event: host.Event{Profile: host.ProfileGoldleaf, SessionID: "session-1",
				State: host.StateCompleted},
		},
		{
			name: "new error",
			event: host.Event{Profile: host.ProfileGoldleaf, SessionID: "session-1",
				State: host.StateServing, Err: errors.New("transfer failed")},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := baseline.SameState(test.event); got != test.want {
				t.Fatalf("SameState() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestEventSameStateComparesErrorText(t *testing.T) {
	left := host.Event{State: host.StateFailed, Err: errors.New("transfer failed")}
	right := host.Event{State: host.StateFailed, Err: errors.New("transfer failed")}
	if !left.SameState(right) {
		t.Fatal("equivalent error text should describe the same observable state")
	}
}
