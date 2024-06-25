package process

var closedOutputChan = make(chan *stateOutput)

func init() {
	close(closedOutputChan)
}
