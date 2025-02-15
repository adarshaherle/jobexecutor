package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/adarshaherle/jobexecutor/proto"
)

func main() {
	// Define flags.
	addr := flag.String("addr", "localhost:50051", "gRPC server address")
	certFile := flag.String("cert", "client.crt", "Path to client certificate")
	keyFile := flag.String("key", "client.key", "Path to client key")
	caFile := flag.String("ca", "ca.crt", "Path to CA certificate")
	flag.Parse()

	// Load client's certificate and key.
	clientCert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		log.Fatalf("failed to load client certificate and key: %v", err)
	}

	// Load the CA certificate using os.ReadFile.
	caCert, err := os.ReadFile(*caFile)
	if err != nil {
		log.Fatalf("failed to read CA certificate: %v", err)
	}
	caPool := x509.NewCertPool()
	if ok := caPool.AppendCertsFromPEM(caCert); !ok {
		log.Fatalf("failed to append CA certificate")
	}

	// Create the TLS configuration for mutual TLS.
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caPool,
	}

	creds := credentials.NewTLS(tlsConfig)

	// Dial the gRPC server using mTLS credentials.
	conn, err := grpc.Dial(*addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatalf("failed to dial server: %v", err)
	}
	defer conn.Close()

	// Create a gRPC client.
	client := pb.NewJobServiceClient(conn)

	// Prepare a context with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a start job request.
	req := &pb.StartJobRequest{
		Command: []string{"echo", "Hello from Level 5 job"},
		Options: &pb.JobOptions{
			CpuLimit:       50000,
			MemoryLimitMb:  128,
			DiskIOLimitBps: 1024 * 1024,
		},
	}

	// Call the StartJob RPC.
	resp, err := client.StartJob(ctx, req)
	if err != nil {
		log.Fatalf("failed to start job: %v", err)
	}

	log.Printf("Started job with ID: %s", resp.JobId)
}
