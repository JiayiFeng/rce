package process

import (
	"context"
	"fmt"
	"github.com/reyoung/rce/protocol"
	"sync"
)

type process struct {
	curState        state
	reqChan         chan *protocol.SpawnRequest
	rspChan         chan *protocol.SpawnResponse
	errChan         chan error
	stateOutputChan <-chan *stateOutput
	complete        sync.WaitGroup
}

func (p *process) PID() string {
	pid, ok := p.curState.(withPID)
	if !ok {
		return ""
	}
	return pid.PID()
}

func (p *process) Kill() error {
	k, ok := p.curState.(withKill)
	if !ok {
		return fmt.Errorf("kill not supported in current state")
	}
	return k.Kill()
}

func (p *process) RequestChan() chan<- *protocol.SpawnRequest {
	return p.reqChan
}

func (p *process) ResponseChan() <-chan *protocol.SpawnResponse {
	return p.rspChan
}

func (p *process) ErrorChan() <-chan error {
	return p.errChan
}

func (p *process) processStateOutput(output *stateOutput) (exit bool) {
	if output.Error != nil {
		p.errChan <- output.Error
		return true
	}

	if output.Response != nil {
		p.rspChan <- output.Response
	}

	if output.Complete {
		return true
	}
	return false
}

func (p *process) readAllStateOutput() (exit bool) {
	for output := range p.stateOutputChan {
		exit = exit || p.processStateOutput(output)
	}
	return
}

func (p *process) switchState(newState state) (exit bool) {
	// switching to new state
	_ = p.curState.Close() // close previous state

	if p.stateOutputChan != nil {
		if p.readAllStateOutput() {
			return true
		}
	}

	p.stateOutputChan = newState.Output()
	p.curState = newState
	return false

}

func (p *process) step(ctx context.Context) (exit bool) {
	select {
	case req := <-p.reqChan:
		newState, err := p.curState.ProcessEvent(ctx, req)
		if err != nil {
			p.errChan <- err
			return true
		}
		if newState != nil {
			exit = p.switchState(newState)
			if exit {
				return true
			}
		}

	case output, ok := <-p.stateOutputChan:
		if !ok { // read all state output, then p.stateOutputChan can be nil
			p.stateOutputChan = nil
			return false
		}
		exit = p.processStateOutput(output)
		return exit
	}

	return false
}

func (p *process) loop(ctx context.Context) {
	defer func() {
		close(p.errChan)
	}()
	for !p.step(ctx) {
	}
}

func (p *process) wait() {
	p.complete.Wait()
}

func New(ctx context.Context) Process {
	p := &process{
		curState: &initState{},
		reqChan:  make(chan *protocol.SpawnRequest),
		rspChan:  make(chan *protocol.SpawnResponse),
		errChan:  make(chan error),
	}
	p.complete.Add(1)
	go func() {
		defer p.complete.Done()
		p.loop(ctx)
	}()
	return p
}
