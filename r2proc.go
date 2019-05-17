package radareutil

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sync"
)

// TODO: stderr. Holding onto stderr will require something
//  constantly read it. Failure to do so will lead to radare2
//  not producing any output.
type r2Proc struct {
	config  *Radare2Config
	mutex   *sync.Mutex
	state   State
	stopped chan StoppedInfo
	cmd     *exec.Cmd
	stdin   io.Writer
	stdout  io.Reader
}

func (o *r2Proc) Status() Status {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	return Status{
		State: o.state,
	}
}

func (o *r2Proc) OnStopped() chan StoppedInfo {
	return o.stopped
}

func (o *r2Proc) Start(mode Mode) error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	if o.state == Running {
		return fmt.Errorf("radare2 process is already running")
	}

	err := o.config.Validate()
	if err != nil {
		return err
	}

	args, err := o.config.Args(mode)
	if err != nil {
		return err
	}

	radare := exec.Command(o.config.ExecutablePath, args...)
	radare.Dir = filepath.Dir(o.config.ExecutablePath)
	radare.SysProcAttr = radareSysProcAttr()

	stdin, err := radare.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe - %s", err.Error())
	}

	stdout, err := radare.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe - %s", err.Error())
	}

	err = radare.Start()
	if err != nil {
		return fmt.Errorf("failed to start radare - %s", err.Error())
	}

	var output *syncBuffer
	if o.config.SaveOutput {
		output = newSyncBuffer()
		o.stdout = io.TeeReader(stdout, output)
	} else {
		o.stdout = stdout
	}

	o.state = Running
	o.cmd = radare
	o.stdin = stdin

	go o.monitor(output)

	return nil
}

func (o *r2Proc) monitor(output *syncBuffer) {
	err := o.cmd.Wait()

	o.mutex.Lock()

	var info StoppedInfo

	if output != nil {
		info.out = output.String()
	}

	if o.state != Stopped {
		o.state = Dead
		info.err = err
	}

	select {
	case o.stopped <- info:
	default:
	}

	o.cmd = nil

	o.mutex.Unlock()
}

func (o *r2Proc) Kill() {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	if o.state != Running {
		return
	}

	o.state = Stopped

	o.cmd.Process.Kill()
}

type syncBuffer struct {
	mutex *sync.Mutex
	buff  *bytes.Buffer
}

func (o *syncBuffer) Write(p []byte) (n int, err error) {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	return o.buff.Write(p)
}

func (o *syncBuffer) String() string {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	return o.buff.String()
}

func newSyncBuffer() *syncBuffer {
	return &syncBuffer{
		mutex: &sync.Mutex{},
		buff:  bytes.NewBuffer(nil),
	}
}

func newR2Proc(config *Radare2Config) (*r2Proc, error) {
	return &r2Proc{
		config:  config,
		mutex:   &sync.Mutex{},
		state:   Stopped,
		stopped: make(chan StoppedInfo),
	}, nil
}
