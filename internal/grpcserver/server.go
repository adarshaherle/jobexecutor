package grpcserver

import (
	"context"
	"fmt"

	pb "github.com/adarshaherle/jobexecutor/proto"
	"github.com/adarshaherle/jobexecutor/internal/job"
)

type jobServiceServer struct {
	pb.UnimplementedJobServiceServer
	mgr *job.Manager
}

func NewJobServiceServer() pb.JobServiceServer {
	return &jobServiceServer{
		mgr: job.NewManager(),
	}
}

func (s *jobServiceServer) StartJob(ctx context.Context, req *pb.StartJobRequest) (*pb.StartJobResponse, error) {
	if len(req.Command) == 0 {
		return nil, fmt.Errorf("command cannot be empty")
	}
	opts := job.JobOptions{
		CPULimit:    int(req.Options.CpuLimit),
		MemoryLimit: int(req.Options.MemoryLimitMb),
		DiskIOLimit: int(req.Options.DiskIOLimitBps),
	}
	jobID, err := s.mgr.StartJob(req.Command, opts)
	if err != nil {
		return nil, err
	}
	return &pb.StartJobResponse{JobId: jobID}, nil
}

func (s *jobServiceServer) StopJob(ctx context.Context, req *pb.StopJobRequest) (*pb.StopJobResponse, error) {
	if err := s.mgr.StopJob(req.JobId); err != nil {
		return nil, err
	}
	return &pb.StopJobResponse{Success: true}, nil
}

func (s *jobServiceServer) GetJobStatus(ctx context.Context, req *pb.JobStatusRequest) (*pb.JobStatusResponse, error) {
	status, exitCode, err := s.mgr.GetStatus(req.JobId)
	if err != nil {
		return nil, err
	}
	return &pb.JobStatusResponse{
		JobId:    req.JobId,
		Status:   status.String(),
		ExitCode: int32(exitCode),
	}, nil
}

func (s *jobServiceServer) StreamOutput(req *pb.JobOutputRequest, stream pb.JobService_StreamOutputServer) error {
	s.mgr.mu.Lock()
	job, ok := s.mgr.jobs[req.JobId]
	s.mgr.mu.Unlock()
	if !ok {
		return fmt.Errorf("job not found")
	}
	sub := job.Subscribe()
	for chunk := range sub {
		if err := stream.Send(&pb.JobOutputChunk{Data: chunk}); err != nil {
			return err
		}
	}
	return nil
}
