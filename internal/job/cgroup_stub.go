//go:build !linux
// +build !linux

package job

import "fmt"

// Export CgroupBasePath for consistency.
var CgroupBasePath = ""

// ResourceLimits redefined for stub.
type ResourceLimits struct {
    CPUQuotaPercent  int
    MemoryLimitBytes int64
    BlockIOWeight    int
}

// ApplyCgroupLimits is a no-op on non-Linux systems.
func ApplyCgroupLimits(jobID string, pid int, limits ResourceLimits) error {
    fmt.Printf("Warning: cgroups not supported on this platform. Job %s running without limits.\n", jobID)
    return nil
}

// RemoveCgroup is a no-op.
func RemoveCgroup(jobID string) error {
    return nil
}
