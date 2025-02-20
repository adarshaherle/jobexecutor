//go:build linux
// +build linux

package job

import (
    "fmt"
    "os"
    "path/filepath"
    "strconv"
)

// CgroupBasePath is set from the environment if provided, else defaults to /sys/fs/cgroup.
var CgroupBasePath = func() string {
    if v := os.Getenv("CGROUP_BASE"); v != "" {
        return v
    }
    return "/sys/fs/cgroup"
}()

type ResourceLimits struct {
    CPUQuotaPercent  int   // e.g., 50 means 50%
    MemoryLimitBytes int64 // e.g., 100*1024*1024 for 100MB
    BlockIOWeight    int   // e.g., 500 out of 1000
}

func ApplyCgroupLimits(jobID string, pid int, limits ResourceLimits) error {
    // If disabled, skip cgroup application.
    if os.Getenv("DISABLE_CGROUP") == "true" {
        return nil
    }
    cgPath := filepath.Join(CgroupBasePath, "jobexecutor", jobID)
    if err := os.MkdirAll(cgPath, 0755); err != nil {
        return fmt.Errorf("failed to create cgroup directory: %w", err)
    }
    if limits.CPUQuotaPercent > 0 && limits.CPUQuotaPercent <= 100 {
        period := int64(100000)
        quota := period * int64(limits.CPUQuotaPercent) / 100
        cpuMaxPath := filepath.Join(cgPath, "cpu.max")
        if err := os.WriteFile(cpuMaxPath, []byte(fmt.Sprintf("%d %d", quota, period)), 0644); err != nil {
            return fmt.Errorf("failed to write cpu.max: %w", err)
        }
    }
    if limits.MemoryLimitBytes > 0 {
        memMaxPath := filepath.Join(cgPath, "memory.max")
        if err := os.WriteFile(memMaxPath, []byte(strconv.FormatInt(limits.MemoryLimitBytes, 10)), 0644); err != nil {
            return fmt.Errorf("failed to write memory.max: %w", err)
        }
    }
    if limits.BlockIOWeight > 0 {
        ioMaxPath := filepath.Join(cgPath, "io.max")
        ioLimit := fmt.Sprintf("rbps=%d wbps=%d", limits.BlockIOWeight*1024, limits.BlockIOWeight*1024)
        if err := os.WriteFile(ioMaxPath, []byte(ioLimit), 0644); err != nil {
            return fmt.Errorf("failed to write io.max: %w", err)
        }
    }
    procsPath := filepath.Join(cgPath, "cgroup.procs")
    if err := os.WriteFile(procsPath, []byte(strconv.Itoa(pid)), 0644); err != nil {
        return fmt.Errorf("failed to write cgroup.procs: %w", err)
    }
    return nil
}

func RemoveCgroup(jobID string) error {
    cgPath := filepath.Join(CgroupBasePath, "jobexecutor", jobID)
    return os.RemoveAll(cgPath)
}
