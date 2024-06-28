package main

import (
	"context"
	"emperror.dev/emperror"
	"fmt"
	"github.com/creack/pty"
	"github.com/docopt/docopt-go"
	"github.com/reyoung/rce/protocol"
	"golang.org/x/term"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

const docs = `Remote Code Executor Client

Usage:
    rce_client [--with-stdin] [--env=<e>]... [--pid-file=<p>]
        [--upload=<u>]... [--dir=<dir>] --address=<a> -- <command> [<args>]...
    rce_client -h | --help
    rce_client --version

Options:
    -h --help                 Show this screen.
    --version                 Show version.
    --upload=<u>              Upload local file to remote. format are "local_path:remote_path".
    --dir=<dir>               Remote working directory, empty use temp dir.
    --address=<a>             Remote server address.
    --env=<e>                 Environment variables. format are "key=value".
    --with-stdin              With stdin.
    --pid-file=<p>            Pid file.
    <command>                 Command to run.
    <args>                    Arguments of command.
`

func panic2[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func prepareHeadFrame(arguments docopt.Opts) (h *protocol.SpawnRequest_Head) {
	h = &protocol.SpawnRequest_Head{}
	envs := arguments["--env"].([]string)
	for _, env := range envs {
		kvs := strings.SplitN(env, "=", 2)
		if len(kvs) != 2 {
			panic(fmt.Sprintf("invalid env, %s", env))
		}
		h.Envs = append(h.Envs, &protocol.SpawnRequest_Head_Env{
			Key:   kvs[0],
			Value: kvs[1],
		})
	}

	dirIface := arguments["--dir"]
	if dirIface != nil {
		dir := dirIface.(string)
		h.Path = dir
	}

	if arguments["--with-stdin"].(bool) {
		h.HasStdin = true
	}

	command := arguments["<command>"].(string)
	h.Command = command
	h.Args = arguments["<args>"].([]string)

	if term.IsTerminal(0) && h.HasStdin {
		rows, cols, err := pty.Getsize(os.Stdin)
		if err != nil {
			panic(fmt.Errorf("failed to get terminal size: %w", err))
		}
		h.WindowSize = &protocol.WindowSize{
			Row: uint32(rows),
			Col: uint32(cols),
		}
		h.AllocatePty = true
	}

	return h
}

const sendBufSize = 4096

func doUploadFile(local, remote string, client protocol.RemoteCodeExecutor_SpawnClient, executable bool) {
	f := panic2(os.Open(local))

	var buf [sendBufSize]byte
	trun := true
	for {
		n, err := f.Read(buf[:])
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}
		emperror.Panic(client.Send(&protocol.SpawnRequest{Payload: &protocol.SpawnRequest_File_{
			File: &protocol.SpawnRequest_File{
				Filename:   remote,
				Content:    buf[:n],
				Executable: executable,
				Truncate:   trun,
			},
		}}))
		trun = false
	}
}

func doUpload2(local, remote string, client protocol.RemoteCodeExecutor_SpawnClient) {
	fs := panic2(os.Stat(local))
	if fs.IsDir() {
		panic(fmt.Sprintf("upload dir is not supported now, %s", local))
	}

	doUploadFile(local, remote, client, fs.Mode()&0100 != 0)
}

func doUpload(uploads []string, client protocol.RemoteCodeExecutor_SpawnClient) {
	for _, u := range uploads {
		us := strings.SplitN(u, ":", 2)
		if len(us) != 2 {
			panic(fmt.Sprintf("invalid upload, %s", u))
		}
		doUpload2(us[0], us[1], client)
	}
}

func doRCE(arguments docopt.Opts, rceClient protocol.RemoteCodeExecutorClient, pid *string) int {
	cli := panic2(rceClient.Spawn(context.Background()))
	emperror.Panic(cli.Send(&protocol.SpawnRequest{
		Payload: &protocol.SpawnRequest_Head_{Head: prepareHeadFrame(arguments)}}))
	doUpload(arguments["--upload"].([]string), cli)
	emperror.Panic(cli.Send(&protocol.SpawnRequest{
		Payload: &protocol.SpawnRequest_Start_{Start: &protocol.SpawnRequest_Start{}}}))

	var complete sync.WaitGroup

	if arguments["--with-stdin"].(bool) {
		complete.Add(1)
		go func() {
			defer complete.Done()
			// read stdin and send to remote
			var buf [sendBufSize]byte
			for {
				n, err := os.Stdin.Read(buf[:])
				if err != nil {
					if err == io.EOF {
						emperror.Panic(cli.Send(&protocol.SpawnRequest{
							Payload: &protocol.SpawnRequest_Stdin_{Stdin: &protocol.SpawnRequest_Stdin{
								Eof: true,
							}}}))
						break
					}
					panic(err)
				}
				emperror.Panic(cli.Send(&protocol.SpawnRequest{
					Payload: &protocol.SpawnRequest_Stdin_{Stdin: &protocol.SpawnRequest_Stdin{
						Stdin: buf[:n],
					}}}))
			}
		}()
	}

	for {
		rsp, err := cli.Recv()
		if err != nil {
			if err == io.EOF {
				return -1
			}
			panic(err)
		}
		if *pid == "" {
			*pid = rsp.GetPid().GetId()

			if *pid != "" && arguments["--pid-file"] != nil {
				emperror.Panic(os.WriteFile(arguments["--pid-file"].(string), []byte(*pid), 0600))
			}
		}
		if rsp.GetError() != nil {
			panic(rsp.GetError().Error)
		}
		if rsp.GetExit() != nil {
			return int(rsp.GetExit().Code)
		}
		if rsp.GetStdout() != nil {
			os.Stdout.Write(rsp.GetStdout().Stdout)
		}
		if rsp.GetStderr() != nil {
			os.Stderr.Write(rsp.GetStderr().Stderr)
		}
	}
}

func main() {
	arguments, _ := docopt.ParseArgs(docs, nil, "Remote Code Executor Client 1.0")
	allocateTTY := false
	if arguments["--with-stdin"].(bool) {
		if fd := int(os.Stdin.Fd()); term.IsTerminal(fd) {
			allocateTTY = true
			panic2(term.MakeRaw(fd))
		}
	}
	addr := arguments["--address"].(string)
	client := panic2(grpc.NewClient(addr, grpc.WithCredentialsBundle(insecure.NewBundle())))
	defer client.Close()
	rceClient := protocol.NewRemoteCodeExecutorClient(client)
	pid := ""
	defer func() {
		if pid != "" {
			log.Printf("Killing pid %s", pid)
			_, _ = rceClient.Kill(context.Background(), &protocol.PID{Id: pid})
		}
	}()

	fn := func() {
		errCode := doRCE(arguments, rceClient, &pid)
		os.Exit(errCode)
	}

	if allocateTTY {
		go fn()
		// wait for signal
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
	} else {
		fn()
	}
}
