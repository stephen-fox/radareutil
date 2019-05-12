package radareutil

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
)

const (
	httpServerArg = "-c=h"
)

type HttpServerOptions struct {
	DisableSandbox bool
	DebugPid       int
	Port           int
	// DetachOnStop requires that HttpApi be set.
	DetachOnStop   bool
	HttpApi        HttpApi
}

type defaultHttpServer struct {
	exePath string
	mutex   *sync.Mutex
	server  *exec.Cmd
	options *HttpServerOptions
	state   State
	stopped chan StoppedInfo
}

func (o *defaultHttpServer) Start() error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	return o.startUnsafe()
}

// startUnsafe starts the server without use of the lock.
func (o *defaultHttpServer) startUnsafe() error {
	if o.state == Running {
		return fmt.Errorf("server is already Running")
	}

	var args []string

	if o.options.Port > 0 {
		args = append(args, httpServerArg + strconv.Itoa(o.options.Port))
	} else {
		args = append(args, httpServerArg)
	}

	if o.options.DisableSandbox {
		args = append(args, "-e", "http.sandbox=false")
	}

	if o.options.DebugPid > 0 {
		args = append(args, "-d", strconv.Itoa(o.options.DebugPid))
	} else {
		args = append(args, "--")
	}

	radare := exec.Command(o.exePath, args...)
	radare.Dir = filepath.Dir(o.exePath)

	output := bytes.NewBuffer(nil)
	radare.Stderr = output
	radare.Stdout = output

	err := radare.Start()
	if err != nil {
		return fmt.Errorf("failed to start radare - %s", err.Error())
	}

	o.state = Running
	o.server = radare

	go o.monitor(output)

	return nil
}

func (o *defaultHttpServer) monitor(output *bytes.Buffer) {
	err := o.server.Wait()

	o.mutex.Lock()

	info := StoppedInfo{
		out: output.String(),
	}

	if o.state != Stopped {
		o.state = Dead
		info.err = err
	}

	select {
	case o.stopped <- info:
	default:
	}

	o.server = nil

	o.mutex.Unlock()
}

func (o *defaultHttpServer) Stop() {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	o.stopUnsafe()
}

// stopUnsafe stops the server without use of the lock.
func (o *defaultHttpServer) stopUnsafe() {
	if o.state != Running {
		return
	}

	o.state = Stopped

	if o.options.DetachOnStop && o.options.HttpApi != nil {
		o.options.HttpApi.Exec("dp-")
	}

	o.server.Process.Kill()
}

// TODO: This can race with the 'monitor()' thread.
//  Needs to be improved.
func (o *defaultHttpServer) Restart() error {
	o.mutex.Lock()
	o.stopUnsafe()
	o.mutex.Unlock()

	o.mutex.Lock()
	err := o.startUnsafe()
	o.mutex.Unlock()
	if err != nil {
		return err
	}

	return nil
}

func (o *defaultHttpServer) Options() *HttpServerOptions {
	return o.options
}

func (o *defaultHttpServer) Status() Status {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	return Status{
		State: o.state,
	}
}

func (o *defaultHttpServer) OnStopped() chan StoppedInfo {
	return o.stopped
}

func (o *defaultHttpServer) Execute(command string) (string, error) {
	return o.options.HttpApi.Exec(command)
}
