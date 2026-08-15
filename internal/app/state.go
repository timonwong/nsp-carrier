package app

import (
	"errors"
	"fmt"
	"sync"
)

var (
	ErrInvalidTransition = errors.New("invalid state transition")
	ErrStaleSession      = errors.New("stale session")
)

type State string

const (
	StateIdle             State = "Idle"
	StateWaitingForDevice State = "WaitingForDevice"
	StateConnected        State = "Connected"
	StateServing          State = "Serving"
	StateCompleted        State = "Completed"
	StateDisconnected     State = "Disconnected"
	StateFailed           State = "Failed"
	StateStopping         State = "Stopping"
)

type Snapshot struct {
	State     State
	SessionID string
}

type StateMachine struct {
	mu        sync.Mutex
	state     State
	sessionID string
}

func NewStateMachine() *StateMachine {
	return &StateMachine{state: StateIdle}
}

func (m *StateMachine) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Snapshot{State: m.state, SessionID: m.sessionID}
}

func (m *StateMachine) Transition(next State, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sessionID != "" && m.state != StateDisconnected && sessionID != m.sessionID {
		return ErrStaleSession
	}
	if !allowedTransition(m.state, next) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, m.state, next)
	}
	if next == StateConnected {
		if sessionID == "" {
			return fmt.Errorf("%w: connected state requires session ID", ErrInvalidTransition)
		}
		m.sessionID = sessionID
	}
	m.state = next
	if next == StateIdle {
		m.sessionID = ""
	}
	return nil
}

func allowedTransition(from, to State) bool {
	switch from {
	case StateIdle:
		return to == StateWaitingForDevice
	case StateWaitingForDevice:
		return to == StateConnected || to == StateStopping || to == StateFailed
	case StateConnected:
		return to == StateServing || to == StateCompleted || to == StateDisconnected || to == StateFailed || to == StateStopping
	case StateServing:
		return to == StateCompleted || to == StateDisconnected || to == StateFailed || to == StateStopping
	case StateCompleted, StateFailed:
		return to == StateStopping
	case StateDisconnected:
		return to == StateConnected || to == StateStopping
	case StateStopping:
		return to == StateIdle
	default:
		return false
	}
}
