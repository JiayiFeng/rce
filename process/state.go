package process

import "github.com/reyoung/rce/protocol"

type state interface {
	// ProcessEvent processes the event and returns the new state and an error.
	// new state may be nil if state is not changed
	ProcessEvent(event *protocol.SpawnRequest) (newState state, err error)

	EndOfInput() bool

	Output() <-chan *protocol.SpawnRequest
}
