//go:build !linux
// +build !linux

package job

func SetupCgroup(jobID string, cpuQuota, memLimit, diskIOLimit, pid int) error {
	return nil
}

func CleanupCgroup(jobID string) error {
	return nil
}
