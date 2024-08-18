//go:build !windows
// +build !windows

package radareutil

import (
	"os"
	"os/exec"
	"syscall"
)

func radareSysProcAttr() *syscall.SysProcAttr {
	return nil
}

func radareInterruptProcFunc() (interruptProcFunc, error) {
	return func(cmd *exec.Cmd) error {
		return cmd.Process.Signal(os.Interrupt)
	}, nil
}
