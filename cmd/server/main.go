package main

import (
	"flag"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/adarshaherle/jobexecutor/proto"
	"github.com/adarshaherle/jobexecutor/internal/grpcserver"
)

func main() {
	addr := flag.String("addr", ":50051", "gRPC server address")
	certFile := flag.String("cert", "server.crt", "Path to server certificate")
	keyFile := flag.String("key", "server.key", "Path to server key")
	caFile := flag.String("ca", "ca.crt", "Path to CA certificate")
	flag.Parse()

	creds, err := credentials.NewServerTLSFromFile(*certFile, *keyFile)
	if err != nil {
		log.Fatalf("failed to load TLS credentials: %v", err)
	}

	grpcServer := grpc.NewServer(grpc.Creds(creds))
	pb.RegisterJobServiceServer(grpcServer, grpcserver.NewJobServiceServer())

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", *addr, err)
	}
	log.Printf("gRPC server listening on %s", *addr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
