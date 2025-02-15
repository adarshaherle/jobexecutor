//go:build !linux
// +build !linux

package job

// cgroupBasePath is defined for non-Linux systems as a no-op.
var cgroupBasePath = ""

// SetupCgroup is a no-op on non-Linux systems.
func SetupCgroup(jobID string, cpuQuota, memLimit, diskIOLimit, pid int) error {
	return nil
}

// CleanupCgroup is a no-op on non-Linux systems.
func CleanupCgroup(jobID string) error {
	return nil
}
