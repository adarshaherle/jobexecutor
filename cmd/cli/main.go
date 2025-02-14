package main

import (
	"context"
	"flag"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/adarshaherle/jobexecutor/proto"
)

func main() {
	addr := flag.String("addr", "localhost:50051", "gRPC server address")
	certFile := flag.String("cert", "client.crt", "Path to client certificate")
	keyFile := flag.String("key", "client.key", "Path to client key")
	caFile := flag.String("ca", "ca.crt", "Path to CA certificate")
	flag.Parse()

	creds, err := credentials.NewClientTLSFromFile(*caFile, "")
	if err != nil {
		log.Fatalf("failed to load TLS credentials: %v", err)
	}

	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatalf("failed to dial server: %v", err)
	}
	defer conn.Close()

	client := pb.NewJobServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.StartJobRequest{
		Command: []string{"echo", "Hello from Level 5 job"},
		Options: &pb.JobOptions{
			CpuLimit:         50000,
			MemoryLimitMb:    128,
			DiskIOLimitBps:   1024 * 1024,
		},
	}
	resp, err := client.StartJob(ctx, req)
	if err != nil {
		log.Fatalf("failed to start job: %v", err)
	}
	log.Printf("Started job with ID: %s", resp.JobId)
}
