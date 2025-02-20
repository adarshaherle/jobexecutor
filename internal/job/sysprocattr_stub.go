//go:build !linux
// +build !linux

package job

import "syscall"

// getSysProcAttr returns nil on non-Linux systems.
func getSysProcAttr() *syscall.SysProcAttr {
    return nil
}
