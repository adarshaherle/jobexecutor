//go:build linux
// +build linux

package job

import (
	"fmt"
	"os"
	"path/filepath"
)

var CgroupBasePath = func() string {
	if v := os.Getenv("CGROUP_BASE"); v != "" {
		return v
	}
	return "/sys/fs/cgroup"
}()

// ResourceLimits holds limits for a job.
type ResourceLimits struct {
	CPUQuotaPercent  int   // e.g., 50 for 50%
	MemoryLimitBytes int64 // e.g., 100 * 1024 * 1024 for 100MB
	BlockIOWeight    int   // e.g., 500
}

// ApplyCgroupLimits creates a directory for the job and writes dummy limit values.
func ApplyCgroupLimits(jobID string, pid int, limits ResourceLimits) error {
	cgPath := filepath.Join(CgroupBasePath, "jobexecutor", jobID)
	if err := os.MkdirAll(cgPath, 0755); err != nil {
		return fmt.Errorf("failed to create cgroup directory: %w", err)
	}
	// For demonstration, we write a dummy value to a file.
	cpuMaxPath := filepath.Join(cgPath, "cpu.max")
	dummyVal := fmt.Sprintf("%d %d", limits.CPUQuotaPercent*1000, 100000)
	if err := os.WriteFile(cpuMaxPath, []byte(dummyVal), 0644); err != nil {
		return fmt.Errorf("failed to write cpu.max: %w", err)
	}
	return nil
}

// RemoveCgroup attempts to remove the cgroup directory.
// We first set permissions (Chmod) to ensure removal succeeds.
func RemoveCgroup(jobID string) error {
	cgPath := filepath.Join(CgroupBasePath, "jobexecutor", jobID)
	// Set permissions so that we can remove it.
	if err := os.Chmod(cgPath, 0755); err != nil {
		return fmt.Errorf("failed to chmod cgroup directory: %w", err)
	}
	return os.RemoveAll(cgPath)
}
