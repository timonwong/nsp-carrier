package app_test

import (
	"errors"
	"testing"

	"github.com/timonwong/ya-dbibackend/internal/app"
)

func TestStateMachineOwnsCanonicalSessionTransitions(t *testing.T) {
	machine := app.NewStateMachine()

	steps := []struct {
		state     app.State
		sessionID string
	}{
		{state: app.StateWaitingForDevice},
		{state: app.StateConnected, sessionID: "session-1"},
		{state: app.StateServing, sessionID: "session-1"},
		{state: app.StateCompleted, sessionID: "session-1"},
		{state: app.StateStopping, sessionID: "session-1"},
		{state: app.StateIdle, sessionID: "session-1"},
	}
	for _, step := range steps {
		if err := machine.Transition(step.state, step.sessionID); err != nil {
			t.Fatalf("Transition(%s) error = %v", step.state, err)
		}
	}

	snapshot := machine.Snapshot()
	if snapshot.State != app.StateIdle || snapshot.SessionID != "" {
		t.Fatalf("Snapshot() = %#v", snapshot)
	}
}

func TestStateMachineRejectsInvalidAndStaleTransitions(t *testing.T) {
	machine := app.NewStateMachine()
	if err := machine.Transition(app.StateServing, "session-1"); !errors.Is(err, app.ErrInvalidTransition) {
		t.Fatalf("Idle -> Serving error = %v", err)
	}
	if err := machine.Transition(app.StateWaitingForDevice, ""); err != nil {
		t.Fatal(err)
	}
	if err := machine.Transition(app.StateConnected, "session-1"); err != nil {
		t.Fatal(err)
	}
	if err := machine.Transition(app.StateServing, "stale-session"); !errors.Is(err, app.ErrStaleSession) {
		t.Fatalf("stale session error = %v", err)
	}
}
