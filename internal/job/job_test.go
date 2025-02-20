package job

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func waitForState(jm *JobManager, jobID string, timeout time.Duration) (JobState, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		state, _, _, err := jm.Status(jobID)
		if err != nil {
			return 0, err
		}
		if state != StateRunning {
			return state, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return StateRunning, fmt.Errorf("timeout waiting for job %s to complete", jobID)
}

func TestJobExecution(t *testing.T) {
	jm := NewJobManager()
	if runtime.GOOS == "windows" {
		t.Skip("TestJobExecution skipped on Windows")
	}
	// Use "sh -c" so the shell executes the command.
	jobID, err := jm.Start("sh", []string{"-c", "echo Test Job Execution"})
	if err != nil {
		t.Fatalf("failed to start job: %v", err)
	}
	state, err := waitForState(jm, jobID, 5*time.Second)
	if err != nil {
		t.Fatalf("job did not complete in time: %v", err)
	}
	if state != StateCompleted {
		t.Fatalf("expected state Completed, got %s", state.String())
	}
	_, exitCode, errMsg, err := jm.Status(jobID)
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (error: %s)", exitCode, errMsg)
	}
	output, err := jm.GetOutput(jobID)
	if err != nil {
		t.Fatalf("failed to get output: %v", err)
	}
	if !strings.Contains(output, "Test Job Execution") {
		t.Errorf("output does not contain expected text; got: %s", output)
	}
}

func TestJobStop(t *testing.T) {
	jm := NewJobManager()
	if runtime.GOOS == "windows" {
		t.Skip("TestJobStop skipped on Windows")
	}
	// Use "sh -c" for sleep.
	jobID, err := jm.Start("sh", []string{"-c", "sleep 5"})
	if err != nil {
		t.Fatalf("failed to start job: %v", err)
	}
	state, _, _, err := jm.Status(jobID)
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	if state != StateRunning {
		t.Fatalf("expected job to be running, got %s", state.String())
	}
	if err := jm.Stop(jobID); err != nil {
		t.Fatalf("failed to stop job: %v", err)
	}
	state, err = waitForState(jm, jobID, 5*time.Second)
	if err != nil {
		t.Fatalf("job did not update state after stop: %v", err)
	}
	if state != StateCancelled && state != StateFailed {
		t.Errorf("expected job to be cancelled or failed after stop, got %s", state.String())
	}
}

func TestOutputStreaming(t *testing.T) {
	jm := NewJobManager()
	// Use "sh -c" for echo.
	jobID, err := jm.Start("sh", []string{"-c", "echo Stream Test"})
	if err != nil {
		t.Fatalf("failed to start job: %v", err)
	}
	jobObj, ok := jm.GetJob(jobID)
	if !ok {
		t.Fatalf("job not found")
	}
	time.Sleep(500 * time.Millisecond)
	output := jobObj.GetOutput()
	if !strings.Contains(output, "Stream Test") {
		t.Errorf("expected output to contain 'Stream Test', got: %s", output)
	}
}

func TestCgroupIntegration(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("cgroup integration test runs on Linux only")
	}
	if os.Getenv("DISABLE_CGROUP") == "true" {
		t.Skip("cgroup integration test skipped when cgroups are disabled")
	}
	tempDir, err := os.MkdirTemp("", "cgtest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	original := CgroupBasePath
	CgroupBasePath = filepath.Join(tempDir, "jobexecutor")
	defer func() { CgroupBasePath = original }()

	jm := NewJobManager()
	// Use "sh -c" for sleep.
	jobID, err := jm.Start("sh", []string{"-c", "sleep 1"})
	if err != nil {
		t.Fatalf("failed to start job: %v", err)
	}
	cgDir := filepath.Join(CgroupBasePath, "jobexecutor", jobID)
	if _, err := os.Stat(cgDir); err != nil {
		t.Errorf("expected cgroup directory %s to exist, got error: %v", cgDir, err)
	}
	_, err = waitForState(jm, jobID, 5*time.Second)
	if err != nil {
		t.Fatalf("job did not finish in time: %v", err)
	}
	if _, err := os.Stat(cgDir); !os.IsNotExist(err) {
		t.Errorf("expected cgroup directory %s to be removed after job finishes", cgDir)
	}
}
