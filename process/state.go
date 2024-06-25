package process

import (
	"context"
	"errors"
	"github.com/reyoung/rce/protocol"
)

var (
	errStateUnexpectedEvent = errors.New("unexpected event")
)

type stateOutput struct {
	Response *protocol.SpawnResponse
	Error    error
	Complete bool
}

type state interface {
	// ProcessEvent processes the event and returns the new state and an error.
	// new state may be nil if state is not changed
	ProcessEvent(ctx context.Context, event *protocol.SpawnRequest) (newState state, err error)

	Output() <-chan *stateOutput

	Close() error
}

type withPID interface {
	PID() string
}

type withKill interface {
	Kill() error
}
