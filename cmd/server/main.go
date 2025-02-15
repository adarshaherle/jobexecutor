package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/adarshaherle/jobexecutor/internal/grpcserver"
	pb "github.com/adarshaherle/jobexecutor/proto"
)

func main() {
	addr := flag.String("addr", ":50051", "gRPC server address")
	certFile := flag.String("cert", "server.crt", "Path to server certificate")
	keyFile := flag.String("key", "server.key", "Path to server key")
	caFile := flag.String("ca", "ca.crt", "Path to CA certificate")
	flag.Parse()

	// Load server certificate and key.
	serverCert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		log.Fatalf("failed to load server certificate and key: %v", err)
	}

	// Load CA certificate to verify client certificates.
	caCert, err := os.ReadFile(*caFile)
	if err != nil {
		log.Fatalf("failed to read CA certificate: %v", err)
	}
	caPool := x509.NewCertPool()
	if ok := caPool.AppendCertsFromPEM(caCert); !ok {
		log.Fatalf("failed to append CA certificate")
	}

	// Configure TLS to require and verify client certificates.
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert, // Require mTLS.
		ClientCAs:    caPool,
	}

	creds := credentials.NewTLS(tlsConfig)

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
