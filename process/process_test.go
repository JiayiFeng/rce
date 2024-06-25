package process

import (
	"context"
	"github.com/reyoung/rce/protocol"
	"sync"
	"testing"
)

func TestProcess(t *testing.T) {
	p := New(context.Background())
	var complete sync.WaitGroup
	complete.Add(2)
	go func() {
		defer complete.Done()
		p.RequestChan() <- &protocol.SpawnRequest{
			Payload: &protocol.SpawnRequest_Head_{
				Head: &protocol.SpawnRequest_Head{
					Command: "ls",
					Args:    []string{"-lha"},
					Path:    "",
				},
			},
		}
		p.RequestChan() <- &protocol.SpawnRequest{
			Payload: &protocol.SpawnRequest_Start_{Start: &protocol.SpawnRequest_Start{}}}
	}()

	go func() {
		defer complete.Done()
		for {
			select {
			case rsp := <-p.ResponseChan():
				t.Log(rsp.String())
			case err, ok := <-p.ErrorChan():
				if !ok {
					return
				}
				t.Error(err)
			}
		}
	}()

	complete.Wait()
	p.Kill()
	p.wait()
}
