//go:build linux
// +build linux

package job

import "syscall"

// getSysProcAttr returns Linux namespace flags using cgroup and namespace isolation.
func getSysProcAttr() *syscall.SysProcAttr {
    return &syscall.SysProcAttr{
        Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
    }
}
