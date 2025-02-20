package job

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"

)

// JobState represents the current state of a job.
type JobState int

const (
	StateRunning JobState = iota
	StateCompleted
	StateFailed
	StateCancelled
)

func (s JobState) String() string {
	switch s {
	case StateRunning:
		return "running"
	case StateCompleted:
		return "completed"
	case StateFailed:
		return "failed"
	case StateCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// Job holds information about a job.
type Job struct {
	ID       string
	Cmd      *exec.Cmd
	State    JobState
	ExitCode int
	Err      error

	outputMu   sync.Mutex
	outputCond *sync.Cond
	// We'll store output lines.
	OutputBuf []string

	// Subscribers for real‑time output.
	subMu       sync.Mutex
	subscribers []chan string
}

func newJob(id string, cmd *exec.Cmd) *Job {
	job := &Job{
		ID:        id,
		Cmd:       cmd,
		State:     StateRunning,
		OutputBuf: make([]string, 0),
	}
	job.outputCond = sync.NewCond(&job.outputMu)
	return job
}

// appendOutput appends a line to the buffer and notifies subscribers.
func (j *Job) appendOutput(line string) {
	j.outputMu.Lock()
	j.OutputBuf = append(j.OutputBuf, line)
	j.outputCond.Broadcast()
	j.outputMu.Unlock()

	j.subMu.Lock()
	for _, sub := range j.subscribers {
		// Non-blocking send.
		select {
		case sub <- line:
		default:
		}
	}
	j.subMu.Unlock()
}

// Subscribe returns a channel that receives new output lines.
func (j *Job) Subscribe() <-chan string {
	j.subMu.Lock()
	defer j.subMu.Unlock()
	ch := make(chan string, 100)
	// Optionally, send already buffered output.
	j.outputMu.Lock()
	for _, line := range j.OutputBuf {
		ch <- line
	}
	j.outputMu.Unlock()
	j.subscribers = append(j.subscribers, ch)
	return ch
}

// GetOutput returns all buffered output as a single string.
func (j *Job) GetOutput() string {
	j.outputMu.Lock()
	defer j.outputMu.Unlock()
	return fmt.Sprint(j.OutputBuf)
}

// finish updates the job state and closes subscriber channels.
func (j *Job) finish(exitCode int, err error) {
	j.outputMu.Lock()
	if err != nil {
		j.State = StateFailed
	} else {
		j.State = StateCompleted
	}
	j.ExitCode = exitCode
	j.Err = err
	j.outputCond.Broadcast()
	j.outputMu.Unlock()

	j.subMu.Lock()
	for _, ch := range j.subscribers {
		close(ch)
	}
	j.subscribers = nil
	j.subMu.Unlock()
}

// JobManager manages jobs.
type JobManager struct {
	mu     sync.Mutex
	jobs   map[string]*Job
	nextID int
}

func NewJobManager() *JobManager {
	return &JobManager{jobs: make(map[string]*Job)}
}

// Start launches a new job.
func (m *JobManager) Start(command string, args []string) (string, error) {
	if command == "" {
		return "", fmt.Errorf("command cannot be empty")
	}
	cmd := exec.Command(command, args...)
	cmd.SysProcAttr = getSysProcAttr()

	// Create a pipe to capture output.
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start command: %w", err)
	}

	m.mu.Lock()
	jobID := fmt.Sprintf("job-%d", m.nextID)
	m.nextID++
	job := newJob(jobID, cmd)
	m.jobs[jobID] = job
	m.mu.Unlock()

	// Apply cgroup limits (if enabled).
	limits := ResourceLimits{
		CPUQuotaPercent:  50,
		MemoryLimitBytes: 100 * 1024 * 1024,
		BlockIOWeight:    500,
	}
	if err := ApplyCgroupLimits(jobID, cmd.Process.Pid, limits); err != nil {
		log.Printf("Warning: cgroup limits not applied: %v", err)
	}

	// Use a scanner to read output continuously.
	go func(j *Job) {
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			line := scanner.Text() + "\n"
			j.appendOutput(line)
		}
		if err := scanner.Err(); err != nil {
			log.Printf("Error reading output for job %s: %v", jobID, err)
		}

		// Wait for the command to finish.
		err := cmd.Wait()
		exitCode := 0
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				exitCode = ee.ExitCode()
			} else {
				exitCode = -1
			}
		}
		j.finish(exitCode, err)
		// Close the writer end.
		pw.Close()
	}(job)

	return jobID, nil
}

// Stop terminates a running job.
func (m *JobManager) Stop(jobID string) error {
	m.mu.Lock()
	job, ok := m.jobs[jobID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("job %s not found", jobID)
	}
	if job.Cmd.Process == nil {
		return fmt.Errorf("no process found for job %s", jobID)
	}
	return job.Cmd.Process.Kill()
}

// Status returns the current state of a job.
func (m *JobManager) Status(jobID string) (JobState, int, string, error) {
	m.mu.Lock()
	job, ok := m.jobs[jobID]
	m.mu.Unlock()
	if !ok {
		return 0, -1, "", fmt.Errorf("job %s not found", jobID)
	}
	job.outputMu.Lock()
	defer job.outputMu.Unlock()
	errMsg := ""
	if job.Err != nil {
		errMsg = job.Err.Error()
	}
	return job.State, job.ExitCode, errMsg, nil
}

// GetOutput returns the complete output of a job.
func (m *JobManager) GetOutput(jobID string) (string, error) {
	m.mu.Lock()
	job, ok := m.jobs[jobID]
	m.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("job %s not found", jobID)
	}
	return job.GetOutput(), nil
}

// GetJob returns the job pointer.
func (m *JobManager) GetJob(jobID string) (*Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[jobID]
	return job, ok
}
