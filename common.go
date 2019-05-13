package radareutil

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sync"
)

type Mode string

const (
	Unset Mode = ""
	Cli   Mode = "cli"
	Http  Mode = "http"
)

type State string

func (o State) String() string {
	return string(o)
}

const (
	Stopped State = "stopped"
	Dead    State = "dead"
	Running State = "running"
)

type Api interface {
	Start() error
	Kill()
	OnStopped() chan StoppedInfo
	Status() Status
	Execute(command string) (string, error)
}

type Status struct {
	State State
}

type StoppedInfo struct {
	err error
	out string
}

func (o *StoppedInfo) Err() error {
	return o.err
}

func (o *StoppedInfo) CombinedOutput() string {
	return o.out
}

type Radare2Config struct {
	Mode               Mode
	ExecutablePath     string
	DoNotTrimOutput    bool
	SaveOutput         bool
	DebugPid           int
	DisableHttpSandbox bool
	HttpPort           int
	DetachOnStop       bool
}

func (o *Radare2Config) Validate() error {
	if o.Mode == Unset {
		return fmt.Errorf("'Mode' must be set")
	}

	exePathFinal, err := fullyQualifiedBinaryPath(o.ExecutablePath)
	if err != nil {
		return err
	}

	o.ExecutablePath = exePathFinal

	return nil
}

func (o *Radare2Config) Args() []string {
	var args []string

	switch o.Mode {
	case Cli:
		args = append(args, "-q0")
	case Http:
		if o.HttpPort > 0 {
			args = append(args, fmt.Sprintf("%s%d", httpServerArg, o.HttpPort))
		} else {
			args = append(args, httpServerArg)
		}

		if o.DisableHttpSandbox {
			args = append(args, "-e", "http.sandbox=false")
		}
	}

	if o.DebugPid > 0 {
		args = append(args, "-d", fmt.Sprintf("%d", o.DebugPid))
	} else {
		args = append(args, "--")
	}

	return args
}

type r2Proc struct {
	config  *Radare2Config
	mutex   *sync.Mutex
	state   State
	stopped chan StoppedInfo
	cmd     *exec.Cmd
	stdin   io.Writer
	stdout  io.Reader
	stderr  io.Reader
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

func (o *r2Proc) Start() error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	if o.state == Running {
		return fmt.Errorf("radare2 process is already running")
	}

	err := o.config.Validate()
	if err != nil {
		return err
	}

	radare := exec.Command(o.config.ExecutablePath, o.config.Args()...)
	radare.Dir = filepath.Dir(o.config.ExecutablePath)

	stdin, err := radare.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe - %s", err.Error())
	}

	stdout, err := radare.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe - %s", err.Error())
	}

	stderr, err := radare.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe - %s", err.Error())
	}

	err = radare.Start()
	if err != nil {
		return fmt.Errorf("failed to start radare - %s", err.Error())
	}

	var output *bytes.Buffer
	if o.config.SaveOutput {
		output = bytes.NewBuffer(nil)
		writeMutex := &sync.Mutex{}
		w := newSyncBuffer(output, writeMutex)
		o.stderr = io.TeeReader(stderr, w)
		o.stdout = io.TeeReader(stdout, w)
	} else {
		o.stderr = stderr
		o.stdout = stdout
	}

	o.state = Running
	o.cmd = radare
	o.stdin = stdin

	go o.monitor(output)

	return nil
}

func (o *r2Proc) monitor(output *bytes.Buffer) {
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

func newSyncBuffer(buffer *bytes.Buffer, mutex *sync.Mutex) *syncBuffer {
	return &syncBuffer{
		mutex: mutex,
		buff:  buffer,
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
