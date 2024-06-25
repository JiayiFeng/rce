package process

import (
	"context"
	"fmt"
	"github.com/reyoung/rce/protocol"
)

// initState is the initial state of the process.
type initState struct {
}

func (s *initState) Output() <-chan *stateOutput {
	return closedOutputChan
}

func (s *initState) Close() error {
	return nil
}

func (s *initState) ProcessEvent(ctx context.Context, event *protocol.SpawnRequest) (newState state, err error) {
	switch v := event.Payload.(type) {
	case *protocol.SpawnRequest_Head_:
		return s.processHead(v.Head)
	default:
		return nil, fmt.Errorf("%w: %T", errStateUnexpectedEvent, event.Payload)
	}
}

func (s *initState) processHead(head *protocol.SpawnRequest_Head) (state, error) {
	return newPreparingState(head)
}
