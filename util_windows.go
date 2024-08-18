package radareutil

import (
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

const (
	kernel32dllName                  = "kernel32.dll"
	freeConsoleProcName              = "FreeConsole"
	attachConsoleProcName            = "AttachConsole"
	setConsoleCtrlHandlerProcName    = "SetConsoleCtrlHandler"
	generateConsoleCtrlEventProcName = "GenerateConsoleCtrlEvent"
)

func radareSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow: true,
	}
}

// Based on work by Stanislav:
// https://stackoverflow.com/a/15281070
func radareInterruptProcFunc() (interruptProcFunc, error) {
	kernel32dll, err := windows.LoadDLL(kernel32dllName)
	if err != nil {
		return nil, err
	}

	freeConsoleProc, err := getProcedure(freeConsoleProcName, kernel32dll)
	if err != nil {
		return nil, err
	}

	attachConsoleProc, err := getProcedure(attachConsoleProcName, kernel32dll)
	if err != nil {
		return nil, err
	}

	setConsoleCtrlHandlerProc, err := getProcedure(setConsoleCtrlHandlerProcName, kernel32dll)
	if err != nil {
		return nil, err
	}

	generateConsoleCtrlEventProc, err := getProcedure(generateConsoleCtrlEventProcName, kernel32dll)
	if err != nil {
		return nil, err
	}

	return func(cmd *exec.Cmd) error {
		// TODO: Error details.
		r, _, err := freeConsoleProc.Call()
		if r == 0 {
			return err
		}

		r, _, err = attachConsoleProc.Call(uintptr(cmd.Process.Pid))
		if r == 0 {
			return err
		}

		r, _, err = setConsoleCtrlHandlerProc.Call(0, uintptr(1))
		if r == 0 {
			return err
		}

		r, _, err = generateConsoleCtrlEventProc.Call(0, 0)
		if r == 0 {
			return err
		}

		r, _, err = freeConsoleProc.Call()
		if r == 0 {
			return err
		}

		// TODO: Need a call back or something here. This sleep is
		//  a super hack.
		time.Sleep(1 * time.Second)

		r, _, err = setConsoleCtrlHandlerProc.Call(0, uintptr(0))
		if r == 0 {
			return err
		}

		return nil
	}, nil
}

func getProcedure(procedureName string, dll *windows.DLL) (*windows.Proc, error) {
	proc, err := dll.FindProc(procedureName)
	if err != nil {
		return nil, err
	}

	return proc, nil
}
