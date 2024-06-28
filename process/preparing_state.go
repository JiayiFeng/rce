package process

import (
	"context"
	"fmt"
	"github.com/reyoung/rce/protocol"
	"log"
	"os"
	"path"
	"strings"
)

type preparingState struct {
	head      *protocol.SpawnRequest_Head
	cleanPath bool
}

func (p *preparingState) ProcessEvent(ctx context.Context, event *protocol.SpawnRequest) (newState state, err error) {
	switch v := event.Payload.(type) {
	case *protocol.SpawnRequest_Start_:
		return p.processStartEvent(ctx, v.Start)
	case *protocol.SpawnRequest_File_:
		err = p.processFileEvent(v.File)
		if err != nil {
			return nil, fmt.Errorf("failed to process file event: %w", err)
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("%w: %T", errStateUnexpectedEvent, event.Payload)
	}
}

func (p *preparingState) processFileEvent(file *protocol.SpawnRequest_File) (err error) {
	if !strings.HasPrefix(file.Filename, "/") {
		file.Filename = path.Join(p.head.Path, file.Filename)
	}
	flag := os.O_CREATE | os.O_WRONLY
	if file.Truncate {
		flag |= os.O_TRUNC
	} else {
		flag |= os.O_APPEND
	}

	var perm os.FileMode = 0600
	if file.Executable {
		perm = 0700
	}
	err = os.MkdirAll(path.Dir(file.Filename), 0700)
	if err != nil {
		return fmt.Errorf("failed to create dir %s: %w", path.Dir(file.Filename), err)
	}
	log.Printf("Creating file %s", file.Filename)

	of, err := os.OpenFile(file.Filename, flag, perm)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", file.Filename, err)
	}
	defer of.Close()
	_, err = of.Write(file.Content)
	if err != nil {
		return fmt.Errorf("failed to write to file %s: %w", file.Filename, err)
	}
	return nil
}

func (p *preparingState) processStartEvent(
	ctx context.Context, start *protocol.SpawnRequest_Start) (newState state, err error) {
	return newRunningState(ctx, p.head)
}

func (p *preparingState) Output() <-chan *stateOutput {
	return closedOutputChan
}

func (p *preparingState) Close() error {
	if p.cleanPath {
		err := os.RemoveAll(p.head.Path)
		if err != nil {
			return fmt.Errorf("failed to remove dir: %w", err)
		}
	}
	return nil
}

func newPreparingState(head *protocol.SpawnRequest_Head) (*preparingState, error) {
	// creating cwd
	cleanPath := false
	if head.Path == "" {
		cleanPath = true
		tmpDir, err := os.MkdirTemp("", "rce")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp dir: %w", err)
		}
		head.Path = tmpDir
	}

	return &preparingState{
		head:      head,
		cleanPath: cleanPath,
	}, nil
}
