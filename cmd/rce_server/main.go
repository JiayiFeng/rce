package main

import (
	"flag"
	"github.com/reyoung/rce/protocol"
	"github.com/reyoung/rce/server"
	"google.golang.org/grpc"
	"log"
	"net"
)

var (
	flagAddress = flag.String("address", ":8999", "grpc address")
)

func main() {
	flag.Parse()

	lis, err := net.Listen("tcp", *flagAddress)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	svr := grpc.NewServer()
	protocol.RegisterRemoteCodeExecutorServer(svr, &server.Server{})
	log.Printf("server listening at %v\n", lis.Addr())
	if err := svr.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
