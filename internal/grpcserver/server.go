package grpcserver

import (
	"context"
	"log"

	"github.com/adarshaherle/jobexecutor/internal/job"
	pb "github.com/adarshaherle/jobexecutor/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	pb.UnimplementedJobServiceServer
	Manager    *job.JobManager
	AllowedCNs map[string]bool // Optional: for authorization
}

func NewServer(manager *job.JobManager, allowedCNs map[string]bool) *Server {
	return &Server{
		Manager:    manager,
		AllowedCNs: allowedCNs,
	}
}

func (s *Server) StartJob(ctx context.Context, req *pb.JobStartRequest) (*pb.JobStartResponse, error) {
	if req.Command == "" {
		return nil, status.Error(codes.InvalidArgument, "command cannot be empty")
	}
	jobID, err := s.Manager.Start(req.Command, req.Args)
	if err != nil {
		log.Printf("StartJob error: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to start job: %v", err)
	}
	return &pb.JobStartResponse{JobId: jobID}, nil
}

func (s *Server) StopJob(ctx context.Context, req *pb.JobIDRequest) (*pb.JobStopResponse, error) {
	if err := s.Manager.Stop(req.JobId); err != nil {
		return nil, status.Errorf(codes.NotFound, "failed to stop job: %v", err)
	}
	return &pb.JobStopResponse{}, nil
}

func (s *Server) GetStatus(ctx context.Context, req *pb.JobIDRequest) (*pb.JobStatusResponse, error) {
	state, exitCode, errMsg, err := s.Manager.Status(req.JobId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "job not found: %v", err)
	}
	var pbStatus pb.Status
	switch state {
	case job.StateRunning:
		pbStatus = pb.Status_STATUS_RUNNING
	case job.StateCompleted:
		pbStatus = pb.Status_STATUS_COMPLETED
	case job.StateCancelled:
		pbStatus = pb.Status_STATUS_CANCELLED
	case job.StateFailed:
		pbStatus = pb.Status_STATUS_FAILED
	default:
		pbStatus = pb.Status_STATUS_UNKNOWN
	}
	return &pb.JobStatusResponse{
		JobId:        req.JobId,
		Status:       pbStatus,
		ExitCode:     int32(exitCode),
		ErrorMessage: errMsg,
	}, nil
}

func (s *Server) StreamOutput(req *pb.JobOutputRequest, stream pb.JobService_StreamOutputServer) error {
	jobObj, ok := s.Manager.GetJob(req.JobId)
	if !ok {
		return status.Errorf(codes.NotFound, "job not found")
	}
	sub := jobObj.Subscribe()
	// Loop indefinitely, sending new output as it arrives.
	for {
		select {
		case line, ok := <-sub:
			if !ok {
				// Channel closed: job finished.
				return nil
			}
			if err := stream.Send(&pb.JobOutputChunk{Data: []byte(line)}); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}
