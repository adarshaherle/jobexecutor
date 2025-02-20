package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"io/ioutil"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/adarshaherle/jobexecutor/internal/grpcserver"
	"github.com/adarshaherle/jobexecutor/internal/job"
	pb "github.com/adarshaherle/jobexecutor/proto"
)

func main() {
	addr := flag.String("addr", "0.0.0.0:50051", "gRPC server address")
	certFile := flag.String("cert", "server.crt", "Path to server TLS certificate")
	keyFile := flag.String("key", "server.key", "Path to server TLS key")
	caFile := flag.String("ca", "ca.crt", "Path to CA certificate for client verification")
	flag.Parse()

	srvCert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		log.Fatalf("failed to load server certificate and key: %v", err)
	}

	caCert, err := ioutil.ReadFile(*caFile)
	if err != nil {
		log.Fatalf("failed to read CA certificate: %v", err)
	}
	caPool := x509.NewCertPool()
	if ok := caPool.AppendCertsFromPEM(caCert); !ok {
		log.Fatalf("failed to append CA certificate")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{srvCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS12,
	}
	creds := credentials.NewTLS(tlsConfig)

	grpcServer := grpc.NewServer(grpc.Creds(creds))
	manager := job.NewJobManager()
	// For simplicity, allow all authenticated clients.
	jobService := grpcserver.NewServer(manager, nil)
	pb.RegisterJobServiceServer(grpcServer, jobService)

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", *addr, err)
	}
	log.Printf("gRPC server listening on %s", *addr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
