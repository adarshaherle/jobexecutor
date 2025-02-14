package job

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestJobExecution(t *testing.T) {
	mgr := NewManager()
	jobID, err := mgr.StartJob([]string{"echo", "Level5 Test"}, JobOptions{
		CPULimit:    50000,
		MemoryLimit: 128,
		DiskIOLimit: 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("failed to start job: %v", err)
	}
	var status JobStatus
	var exitCode int
	for i := 0; i < 50; i++ {
		status, exitCode, err = mgr.GetStatus(jobID)
		if err != nil {
			t.Fatalf("error getting status: %v", err)
		}
		if status != StatusRunning {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if status != StatusCompleted {
		t.Fatalf("expected completed, got %s", status.String())
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	output, err := mgr.GetOutput(jobID)
	if err != nil {
		t.Fatalf("failed to get output: %v", err)
	}
	if !strings.Contains(output, "Level5 Test") {
		t.Errorf("output does not contain expected text: %s", output)
	}
}

func TestJobStop(t *testing.T) {
	mgr := NewManager()
	jobID, err := mgr.StartJob([]string{"sleep", "5"}, JobOptions{
		CPULimit:    50000,
		MemoryLimit: 128,
		DiskIOLimit: 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("failed to start job: %v", err)
	}
	status, _, _ := mgr.GetStatus(jobID)
	if status != StatusRunning {
		t.Fatalf("job should be running, got %s", status.String())
	}
	if err := mgr.StopJob(jobID); err != nil {
		t.Fatalf("failed to stop job: %v", err)
	}
	for i := 0; i < 50; i++ {
		status, _, _ = mgr.GetStatus(jobID)
		if status != StatusRunning {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if status != StatusStopped && status != StatusFailed {
		t.Errorf("expected stopped/failed, got %s", status.String())
	}
}

func TestOutputStreaming(t *testing.T) {
	mgr := NewManager()
	jobID, err := mgr.StartJob([]string{"echo", "StreamTest"}, JobOptions{
		CPULimit:    50000,
		MemoryLimit: 128,
		DiskIOLimit: 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("failed to start job: %v", err)
	}
	sub := mgr.jobs[jobID].Subscribe()
	time.Sleep(500 * time.Millisecond)
	var out string
loop:
	for {
		select {
		case data, ok := <-sub:
			if !ok {
				break loop
			}
			out += string(data)
		default:
			break loop
		}
	}
	if !strings.Contains(out, "StreamTest") {
		t.Errorf("stream output missing expected text, got: %s", out)
	}
}

func TestCgroupIntegration(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("cgroup integration test runs on Linux only")
	}
	tempDir, err := os.MkdirTemp("", "cgtest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	cgroupBasePath = tempDir
	mgr := NewManager()
	jobID, err := mgr.StartJob([]string{"sleep", "1"}, JobOptions{
		CPULimit:    50000,
		MemoryLimit: 128,
		DiskIOLimit: 1024 * 1024,
	})
	if err != nil {
		t.Fatalf("failed to start job: %v", err)
	}
	cgDir := filepath.Join(tempDir, jobID)
	if _, err := os.Stat(cgDir); err != nil {
		t.Errorf("expected cgroup dir exists, got error: %v", err)
	}
	time.Sleep(2 * time.Second)
	if _, err := os.Stat(cgDir); !os.IsNotExist(err) {
		t.Errorf("expected cgroup dir removed after job, but it exists")
	}
}

// Fuzz test using Go 1.21's built-in fuzzing.
func FuzzGetOutput(f *testing.F) {
	// Seed corpus.
	f.Add("echo", "FuzzTest")
	f.Fuzz(func(t *testing.T, command string, arg string) {
		mgr := NewManager()
		jobID, err := mgr.StartJob([]string{command, arg}, JobOptions{
			CPULimit:    50000,
			MemoryLimit: 128,
			DiskIOLimit: 1024 * 1024,
		})
		if err != nil {
			return
		}
		for i := 0; i < 20; i++ {
			status, _, err := mgr.GetStatus(jobID)
			if err == nil && status != StatusRunning {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		output, err := mgr.GetOutput(jobID)
		if err != nil {
			t.Skip()
		}
		t.Log(output)
	})
}
