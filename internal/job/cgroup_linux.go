//go:build linux
// +build linux

package job

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
)

var cgroupBasePath = "/sys/fs/cgroup/jobexecutor"

// SetupCgroup creates a cgroup for the job and applies resource limits.
func SetupCgroup(jobID string, cpuQuota, memLimit, diskIOLimit, pid int) error {
	cgPath := filepath.Join(cgroupBasePath, jobID)
	if err := os.MkdirAll(cgPath, 0755); err != nil {
		return fmt.Errorf("failed to create cgroup directory: %w", err)
	}
	// CPU limits.
	if cpuQuota > 0 {
		period := "100000" // 100ms period
		if err := ioutil.WriteFile(filepath.Join(cgPath, "cpu.cfs_period_us"), []byte(period), 0644); err != nil {
			return fmt.Errorf("failed to write cpu period: %w", err)
		}
		quotaStr := strconv.Itoa(cpuQuota)
		if err := ioutil.WriteFile(filepath.Join(cgPath, "cpu.cfs_quota_us"), []byte(quotaStr), 0644); err != nil {
			return fmt.Errorf("failed to write cpu quota: %w", err)
		}
	}
	// Memory limit in bytes.
	if memLimit > 0 {
		memBytes := memLimit * 1024 * 1024
		memStr := strconv.Itoa(memBytes)
		if err := ioutil.WriteFile(filepath.Join(cgPath, "memory.limit_in_bytes"), []byte(memStr), 0644); err != nil {
			return fmt.Errorf("failed to write memory limit: %w", err)
		}
	}
	// Disk I/O limits.
	if diskIOLimit > 0 {
		ioLimit := fmt.Sprintf("rbps=%d wbps=%d", diskIOLimit, diskIOLimit)
		if err := ioutil.WriteFile(filepath.Join(cgPath, "io.max"), []byte(ioLimit), 0644); err != nil {
			return fmt.Errorf("failed to write io limit: %w", err)
		}
	}
	// Add process to the cgroup.
	pidStr := strconv.Itoa(pid)
	if err := ioutil.WriteFile(filepath.Join(cgPath, "cgroup.procs"), []byte(pidStr), 0644); err != nil {
		return fmt.Errorf("failed to add pid to cgroup: %w", err)
	}
	return nil
}

// CleanupCgroup removes the cgroup directory.
func CleanupCgroup(jobID string) error {
	cgPath := filepath.Join(cgroupBasePath, jobID)
	return os.RemoveAll(cgPath)
}
