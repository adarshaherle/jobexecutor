//go:build linux
// +build linux

package job

import "syscall"

// getSysProcAttr returns a pointer to syscall.SysProcAttr configured with namespace flags.
func getSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
	}
}
