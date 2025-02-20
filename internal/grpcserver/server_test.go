package grpcserver

import (
    "context"
    "net"
    "testing"
    "time"

    pb "github.com/adarshaherle/jobexecutor/proto"
    "github.com/adarshaherle/jobexecutor/internal/job"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

func startTestGRPCServer(t *testing.T) (pb.JobServiceClient, func()) {
    lis, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        t.Fatalf("failed to listen: %v", err)
    }
    s := grpc.NewServer()
    manager := job.NewJobManager()
    srv := NewServer(manager, nil)
    pb.RegisterJobServiceServer(s, srv)
    go func() {
        if err := s.Serve(lis); err != nil {
            t.Logf("gRPC server error: %v", err)
        }
    }()
    conn, err := grpc.Dial(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        t.Fatalf("failed to dial server: %v", err)
    }
    client := pb.NewJobServiceClient(conn)
    return client, func() {
        s.Stop()
        conn.Close()
    }
}

func TestGRPCStartAndStatus(t *testing.T) {
    client, cleanup := startTestGRPCServer(t)
    defer cleanup()

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    startReq := &pb.JobStartRequest{
        // Use "sh -c" to run the command.
        Command: "sh",
        Args:    []string{"-c", "echo Integration Test"},
    }
    startResp, err := client.StartJob(ctx, startReq)
    if err != nil {
        t.Fatalf("StartJob failed: %v", err)
    }
    if startResp.JobId == "" {
        t.Fatalf("expected non-empty job ID")
    }
    time.Sleep(1 * time.Second)
    statusReq := &pb.JobIDRequest{JobId: startResp.JobId}
    statusResp, err := client.GetStatus(ctx, statusReq)
    if err != nil {
        t.Fatalf("GetStatus failed: %v", err)
    }
    if statusResp.Status != pb.Status_STATUS_COMPLETED {
        t.Errorf("expected status COMPLETED, got %v", statusResp.Status)
    }
}
