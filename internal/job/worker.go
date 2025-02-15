package job

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

// JobStatus represents the state of a job.
type JobStatus int

const (
	StatusPending JobStatus = iota
	StatusRunning
	StatusCompleted
	StatusStopped
	StatusFailed
)

func (s JobStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusRunning:
		return "running"
	case StatusCompleted:
		return "completed"
	case StatusStopped:
		return "stopped"
	case StatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// JobOptions holds resource limits for a job.
type JobOptions struct {
	CPULimit    int // CPU quota in microseconds per period (e.g. 50000 for 50%)
	MemoryLimit int // Memory limit in MB
	DiskIOLimit int // Disk I/O limit in bytes per second (simplified)
}

// Job represents a process running as a job.
type Job struct {
	ID      string
	Command []string
	Status  JobStatus
	Output  bytes.Buffer

	exitCode   int
	startedAt  time.Time
	finishedAt time.Time

	cmd        *exec.Cmd
	cancelFunc context.CancelFunc

	subscribers []chan []byte
	subMu       sync.Mutex

	mu sync.Mutex
}

// Subscribe returns a channel for streaming output.
func (j *Job) Subscribe() chan []byte {
	ch := make(chan []byte, 100)
	j.subMu.Lock()
	j.subscribers = append(j.subscribers, ch)
	j.subMu.Unlock()

	// Send already buffered output.
	go func() {
		j.mu.Lock()
		data := j.Output.Bytes()
		j.mu.Unlock()
		if len(data) > 0 {
			ch <- data
		}
	}()
	return ch
}

func (j *Job) broadcastOutput(data []byte) {
	j.subMu.Lock()
	defer j.subMu.Unlock()
	for _, ch := range j.subscribers {
		select {
		case ch <- data:
		default:
			// Skip if channel is full.
		}
	}
}

func (j *Job) closeSubscribers() {
	j.subMu.Lock()
	defer j.subMu.Unlock()
	for _, ch := range j.subscribers {
		close(ch)
	}
	j.subscribers = nil
}

func (j *Job) captureOutput(pipe io.ReadCloser) {
	defer pipe.Close()
	buf := make([]byte, 1024)
	for {
		n, err := pipe.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			j.mu.Lock()
			j.Output.Write(data)
			j.mu.Unlock()
			j.broadcastOutput(data)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			break
		}
	}
}

// Manager manages jobs.
type Manager struct {
	mu   sync.Mutex
	jobs map[string]*Job
}

// NewManager creates a new Manager instance.
func NewManager() *Manager {
	return &Manager{jobs: make(map[string]*Job)}
}

// GetJob returns the job with the given ID.
func (m *Manager) GetJob(jobID string) (*Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[jobID]
	return job, ok
}

// generateJobID returns a pseudo-unique job ID.
func generateJobID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// StartJob starts a new process with the given command and options.
func (m *Manager) StartJob(command []string, opts JobOptions) (string, error) {
	if len(command) == 0 {
		return "", fmt.Errorf("command cannot be empty")
	}

	jobID := generateJobID()
	ctx, cancel := context.WithCancel(context.Background())

	job := &Job{
		ID:          jobID,
		Command:     command,
		Status:      StatusPending,
		cancelFunc:  cancel,
		subscribers: []chan []byte{},
	}
	m.mu.Lock()
	m.jobs[jobID] = job
	m.mu.Unlock()

	// Set up the command with namespace isolation if available.
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	// Use our helper function to get platform-specific SysProcAttr.
	cmd.SysProcAttr = getSysProcAttr()

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stderr pipe: %w", err)
	}
	job.cmd = cmd

	if err := cmd.Start(); err != nil {
		job.Status = StatusFailed
		return "", fmt.Errorf("failed to start command: %w", err)
	}
	job.Status = StatusRunning
	job.startedAt = time.Now()

	// Set up cgroups for resource control.
	if err := SetupCgroup(jobID, opts.CPULimit, opts.MemoryLimit, opts.DiskIOLimit, cmd.Process.Pid); err != nil {
		_ = cmd.Process.Kill()
		return "", fmt.Errorf("failed to set up cgroup: %w", err)
	}

	go job.captureOutput(stdoutPipe)
	go job.captureOutput(stderrPipe)
	go func(j *Job) {
		err := cmd.Wait()
		j.mu.Lock()
		j.finishedAt = time.Now()
		if err != nil {
			if ctx.Err() != nil {
				j.Status = StatusStopped
			} else {
				j.Status = StatusFailed
			}
		} else {
			j.Status = StatusCompleted
			j.exitCode = cmd.ProcessState.ExitCode()
		}
		j.mu.Unlock()
		j.closeSubscribers()
		_ = CleanupCgroup(jobID)
	}(job)

	return jobID, nil
}

// StopJob stops a running job.
func (m *Manager) StopJob(jobID string) error {
	m.mu.Lock()
	job, ok := m.jobs[jobID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("job not found")
	}
	job.mu.Lock()
	defer job.mu.Unlock()
	if job.Status != StatusRunning {
		return fmt.Errorf("job is not running")
	}
	job.cancelFunc()
	if job.cmd != nil && job.cmd.Process != nil {
		if err := job.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	}
	return nil
}

// GetStatus returns the current status and exit code of a job.
func (m *Manager) GetStatus(jobID string) (JobStatus, int, error) {
	m.mu.Lock()
	job, ok := m.jobs[jobID]
	m.mu.Unlock()
	if !ok {
		return StatusFailed, -1, fmt.Errorf("job not found")
	}
	job.mu.Lock()
	defer job.mu.Unlock()
	return job.Status, job.exitCode, nil
}

// GetOutput returns the accumulated output from a job.
func (m *Manager) GetOutput(jobID string) (string, error) {
	m.mu.Lock()
	job, ok := m.jobs[jobID]
	m.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("job not found")
	}
	job.mu.Lock()
	defer job.mu.Unlock()
	return job.Output.String(), nil
}
