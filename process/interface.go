package process

import "github.com/reyoung/rce/protocol"

type Process interface {
	withPID
	withKill

	RequestChan() chan<- *protocol.SpawnRequest
	ResponseChan() <-chan *protocol.SpawnResponse
	ErrorChan() <-chan error

	// wait the process complete.
	// it seems that this API is not necessary.
	// but useful in unit tests.
	wait()
}
