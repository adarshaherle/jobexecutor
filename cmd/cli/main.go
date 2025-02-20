package main

import (
    "context"
    "crypto/tls"
    "crypto/x509"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "time"

    "github.com/spf13/cobra"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"

    pb "github.com/adarshaherle/jobexecutor/proto"
)

var (
    serverAddr string
    certFile   string
    keyFile    string
    caFile     string
    outputJSON bool
    verbose    bool
)

func main() {
    rootCmd := &cobra.Command{
        Use:   "jobexecutor",
        Short: "Job Executor CLI: manage background jobs via gRPC",
    }
    rootCmd.PersistentFlags().StringVar(&serverAddr, "server", "localhost:50051", "gRPC server address")
    rootCmd.PersistentFlags().StringVar(&certFile, "cert", "client.crt", "Path to client TLS certificate")
    rootCmd.PersistentFlags().StringVar(&keyFile, "key", "client.key", "Path to client TLS key")
    rootCmd.PersistentFlags().StringVar(&caFile, "ca", "ca.crt", "Path to CA certificate")
    rootCmd.PersistentFlags().BoolVarP(&outputJSON, "output", "o", false, "Output in JSON format")
    rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

    rootCmd.AddCommand(startCmd())
    rootCmd.AddCommand(stopCmd())
    rootCmd.AddCommand(statusCmd())
    rootCmd.AddCommand(logsCmd())

    if err := rootCmd.Execute(); err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
}

func newGRPCClient() (pb.JobServiceClient, *grpc.ClientConn, error) {
    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to load client cert and key: %w", err)
    }
    caCertBytes, err := ioutil.ReadFile(caFile)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to read CA cert: %w", err)
    }
    caPool := x509.NewCertPool()
    if ok := caPool.AppendCertsFromPEM(caCertBytes); !ok {
        return nil, nil, fmt.Errorf("failed to append CA cert")
    }
    tlsConfig := &tls.Config{
        Certificates: []tls.Certificate{cert},
        RootCAs:      caPool,
        MinVersion:   tls.VersionTLS12,
    }
    creds := credentials.NewTLS(tlsConfig)
    conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(creds))
    if err != nil {
        return nil, nil, fmt.Errorf("failed to dial server: %w", err)
    }
    return pb.NewJobServiceClient(conn), conn, nil
}

func startCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "start -- <command> [args...]",
        Short: "Start a new job",
        Run: func(cmd *cobra.Command, args []string) {
            if len(args) < 1 {
                fmt.Println("Please provide a command to run.")
                os.Exit(1)
            }
            client, conn, err := newGRPCClient()
            if err != nil {
                log.Fatalf("Error creating gRPC client: %v", err)
            }
            defer conn.Close()

            ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
            defer cancel()

            req := &pb.JobStartRequest{
                Command: args[0],
                Args:    args[1:],
            }
            resp, err := client.StartJob(ctx, req)
            if err != nil {
                log.Fatalf("Error starting job: %v", err)
            }
            if outputJSON {
                b, _ := json.MarshalIndent(resp, "", "  ")
                fmt.Println(string(b))
            } else {
                fmt.Printf("Job started with ID: %s\n", resp.JobId)
            }
        },
    }
}

func stopCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "stop <job_id>",
        Short: "Stop a running job",
        Args:  cobra.ExactArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
            client, conn, err := newGRPCClient()
            if err != nil {
                log.Fatalf("Error creating gRPC client: %v", err)
            }
            defer conn.Close()

            ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
            defer cancel()

            req := &pb.JobIDRequest{JobId: args[0]}
            _, err = client.StopJob(ctx, req)
            if err != nil {
                log.Fatalf("Error stopping job: %v", err)
            }
            fmt.Printf("Job %s stopped successfully\n", args[0])
        },
    }
}

func statusCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "status <job_id>",
        Short: "Get the status of a job",
        Args:  cobra.ExactArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
            client, conn, err := newGRPCClient()
            if err != nil {
                log.Fatalf("Error creating gRPC client: %v", err)
            }
            defer conn.Close()

            ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
            defer cancel()

            req := &pb.JobIDRequest{JobId: args[0]}
            resp, err := client.GetStatus(ctx, req)
            if err != nil {
                log.Fatalf("Error getting status: %v", err)
            }
            if outputJSON {
                b, _ := json.MarshalIndent(resp, "", "  ")
                fmt.Println(string(b))
            } else {
                fmt.Printf("Job %s status: %s (Exit Code: %d, Error: %s)\n",
                    resp.JobId, resp.Status, resp.ExitCode, resp.ErrorMessage)
            }
        },
    }
}

func logsCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "logs <job_id>",
        Short: "Stream output (logs) of a job",
        Args:  cobra.ExactArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
            client, conn, err := newGRPCClient()
            if err != nil {
                log.Fatalf("Error creating gRPC client: %v", err)
            }
            defer conn.Close()

            ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
            defer cancel()

            req := &pb.JobOutputRequest{JobId: args[0]}
            stream, err := client.StreamOutput(ctx, req)
            if err != nil {
                log.Fatalf("Error streaming output: %v", err)
            }
            for {
                chunk, err := stream.Recv()
                if err != nil {
                    break
                }
                fmt.Print(string(chunk.Data))
            }
        },
    }
}
