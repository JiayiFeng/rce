package server

import (
	"context"
	"errors"
	"github.com/reyoung/rce/process"
	"github.com/reyoung/rce/protocol"
	"sync"
)

type Server struct {
	protocol.UnimplementedRemoteCodeExecutorServer

	processes map[string]process.Process
	mutex     sync.RWMutex
}

func forceClose(c chan struct{}) {
	defer func() {
		recover()
	}()
	close(c)
}

type processSetter struct {
	pid string
	s   *Server
	p   process.Process
}

func (p *processSetter) TrySet() {
	if p.pid != "" { // already set
		return
	}
	p.pid = p.p.PID()
	if p.pid == "" { // not started
		return
	}
	p.s.mutex.Lock()
	defer p.s.mutex.Unlock()
	p.s.processes[p.pid] = p.p
}

func (p *processSetter) Unset() {
	if p.pid == "" {
		return
	}
	p.s.mutex.Lock()
	defer p.s.mutex.Unlock()
	delete(p.s.processes, p.pid)
}

func (s *Server) Spawn(svr protocol.RemoteCodeExecutor_SpawnServer) error {
	p := process.New(svr.Context())
	defer func() {
		_ = p.Close()
	}()

	exited := make(chan struct{})
	var complete sync.WaitGroup
	complete.Add(2) // 2 means send/recv

	go func() {
		defer complete.Done()
		for {
			req, err := svr.Recv()
			if err != nil {
				forceClose(exited)
				return
			}
			p.RequestChan() <- req
		}
	}()

	var err error
	pidSetter := &processSetter{s: s, p: p}
	defer pidSetter.Unset()

	go func() {
		defer func() {
			forceClose(exited)
			complete.Done()
		}()
		for {
			var ok bool
			select {
			case rsp := <-p.ResponseChan():
				pidSetter.TrySet()
				err = svr.Send(rsp)
				if err != nil {
					return
				}

			case err, ok = <-p.ErrorChan():
				if !ok {
					return
				}

				rsp := &protocol.SpawnResponse{Payload: &protocol.SpawnResponse_Error{
					Error: &protocol.SpawnResponse_SystemError{Error: err.Error()}}}
				err = errors.Join(err, svr.Send(rsp))
				return
			case <-exited:
				return
			}
		}
	}()
	complete.Wait()
	return err
}

func (s *Server) Kill(ctx context.Context, pid *protocol.PID) (*protocol.KillResponse, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	p, ok := s.processes[pid.Id]
	if !ok {
		return &protocol.KillResponse{Error: "process not found"}, nil
	}
	err := p.Kill()
	if err != nil {
		return &protocol.KillResponse{Error: err.Error()}, nil
	}
	return nil, nil
}
