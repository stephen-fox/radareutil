package radareutil

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

type Mode string

func (o Mode) String() string {
	return string(o)
}

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

type interruptProcFunc func(*exec.Cmd) error

type Api interface {
	Start() error
	Interrupt() error
	Kill()
	OnStopped() chan StoppedInfo
	Status() Status
	Execute(command string) (string, error)
	ExecuteToJson(command string, pointer interface{}) error
	ExecuteToBytes(command string) ([]byte, error)
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
	ExecutablePath     string
	CustomCliArgs      []string
	AdditionalCliArgs  []string
	DoNotTrimOutput    bool
	SaveOutput         bool
	DebugPid           int
	DisableHttpSandbox bool
	HttpPort           int
	DetachOnStop       bool
}

func (o *Radare2Config) Validate() error {
	if o.ExecutablePath == "" {
		return errors.New("executable path is empty")
	}

	return nil
}

func (o *Radare2Config) Args(mode Mode) ([]string, error) {
	if o.CustomCliArgs != nil {
		return o.CustomCliArgs, nil
	}

	var args []string

	switch mode {
	case Cli:
		args = append(args, "-q")
		args = append(args, "-0")
	case Http:
		if o.HttpPort > 0 {
			args = append(args, fmt.Sprintf("%s%d", httpServerArg, o.HttpPort))
		} else {
			args = append(args, httpServerArg)
		}

		if o.DisableHttpSandbox {
			args = append(args, "-e", "http.sandbox=false")
		}
	default:
		return nil, fmt.Errorf("unknown mode '%s'", mode.String())
	}

	if o.DebugPid > 0 {
		args = append(args, "-d", fmt.Sprintf("%d", o.DebugPid))
	}

	if len(o.AdditionalCliArgs) > 0 {
		args = append(args, o.AdditionalCliArgs...)
	}

	return args, nil
}

// TODO: stderr. Holding onto stderr will require something
// constantly read it. Failure to do so will lead to radare2
// not producing any output.
type r2Proc struct {
	config  *Radare2Config
	mutex   *sync.Mutex
	state   State
	stopped chan StoppedInfo
	cmd     *exec.Cmd
	stdin   io.Writer
	stdout  *bufio.Reader
	inter   interruptProcFunc
	stop    chan func()
}

func (o *r2Proc) status() Status {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	return Status{
		State: o.state,
	}
}

func (o *r2Proc) onStopped() chan StoppedInfo {
	return o.stopped
}

func (o *r2Proc) start(mode Mode) error {
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
	radare.SysProcAttr = radareSysProcAttr()

	stdin, err := radare.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe - %s", err.Error())
	}

	stdout, err := radare.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe - %s", err.Error())
	}

	var output *syncBuffer
	if o.config.SaveOutput {
		output = newSyncBuffer()
		o.stdout = bufio.NewReader(io.TeeReader(stdout, output))
	} else {
		o.stdout = bufio.NewReader(stdout)
	}

	err = radare.Start()
	if err != nil {
		return fmt.Errorf("failed to start radare - %s", err.Error())
	}

	o.state = Running
	o.cmd = radare
	o.stdin = stdin

	go o.monitor(output)

	return nil
}

func (o *r2Proc) monitor(output *syncBuffer) {
	procExit := make(chan func() (func(), error))

	go func() {
		err := o.cmd.Wait()
		o.mutex.Lock()
		fn := func() (func(), error) {
			return o.mutex.Unlock, err
		}
		select {
		case procExit <- fn:
		default:
			o.mutex.Unlock()
		}
	}()

	var info StoppedInfo

	select {
	case fn := <-procExit:
		onDone, err := fn()
		if err != nil {
			o.state = Dead
			info.err = err
		} else {
			o.state = Stopped
		}
		defer onDone()
	case onDone := <-o.stop:
		o.state = Stopped
		o.cmd.Process.Kill()
		defer onDone()
	}

	if output != nil {
		info.out = output.String()
	}

	select {
	case o.stopped <- info:
	default:
	}

	o.cmd = nil
}

func (o *r2Proc) interrupt() error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	if o.state != Running {
		return nil
	}

	return o.inter(o.cmd)
}

func (o *r2Proc) kill() {
	o.mutex.Lock()

	if o.state != Running {
		o.mutex.Unlock()
		return
	}

	rejoin := make(chan struct{})

	o.stop <- func() {
		o.mutex.Unlock()
		rejoin <- struct{}{}
	}

	<-rejoin
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
	interruptFunc, err := radareInterruptProcFunc()
	if err != nil {
		return nil, err
	}

	return &r2Proc{
		config:  config,
		mutex:   &sync.Mutex{},
		state:   Stopped,
		stopped: make(chan StoppedInfo),
		inter:   interruptFunc,
		stop:    make(chan func()),
	}, nil
}
