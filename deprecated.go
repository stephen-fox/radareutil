package radareutil

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const (
	cmdSubPath = "/cmd"
)

// Deprecated: Use 'NewCustomHttpServerApi()' instead.
type HttpApiOptions struct {
	Timeout             time.Duration
	DoNotTrimWhiteSpace bool
}

// Deprecated: Use 'NewCustomHttpServerApi()' instead.
type HttpApi interface {
	Exec(command string) (string, error)
}

// Deprecated: Use 'HttpServerApi' instead.
type defaultHttpApi struct {
	httpClient *http.Client
	address    *url.URL
	options    *HttpApiOptions
}

func (o defaultHttpApi) Exec(command string) (string, error) {
	content, err := executeHttpCall(command, o.address, o.httpClient, !o.options.DoNotTrimWhiteSpace)
	if err != nil {
		return string(content), err
	}

	return string(content), nil
}

// Deprecated: Use 'NewCustomHttpServerApi()' instead.
func NewHttpApi(address *url.URL, options *HttpApiOptions) (HttpApi, error) {
	if options.Timeout == 0 {
		options.Timeout = 10 * time.Second
	}

	return defaultHttpApi{
		httpClient: &http.Client{
			Timeout: options.Timeout,
		},
		address: address,
		options: options,
	}, nil
}

// Deprecated: Use 'HttpServerApi' instead.
type HttpServer interface {
	Options() *HttpServerOptions
	Start() error
	Stop()
	OnStopped() chan StoppedInfo
	Restart() error
	Status() Status
	Execute(command string) (string, error)
}

// Deprecated: Use 'Radare2Options' instead.
type HttpServerOptions struct {
	DisableSandbox bool
	DebugPid       int
	Port           int
	// DetachOnStop requires that HttpApi be set.
	DetachOnStop   bool
	HttpApi        HttpApi
}

// Deprecated: This type represents legacy implementation of HTTP server.
type deprecatedHttpServer struct {
	exePath string
	mutex   *sync.Mutex
	server  *exec.Cmd
	options *HttpServerOptions
	state   State
	stopped chan StoppedInfo
}

func (o *deprecatedHttpServer) Start() error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	return o.startUnsafe()
}

// startUnsafe starts the server without use of the lock.
func (o *deprecatedHttpServer) startUnsafe() error {
	if o.state == Running {
		return fmt.Errorf("server is already running")
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

func (o *deprecatedHttpServer) monitor(output *bytes.Buffer) {
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

func (o *deprecatedHttpServer) Stop() {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	o.stopUnsafe()
}

// stopUnsafe stops the server without use of the lock.
func (o *deprecatedHttpServer) stopUnsafe() {
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
func (o *deprecatedHttpServer) Restart() error {
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

func (o *deprecatedHttpServer) Options() *HttpServerOptions {
	return o.options
}

func (o *deprecatedHttpServer) Status() Status {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	return Status{
		State: o.state,
	}
}

func (o *deprecatedHttpServer) OnStopped() chan StoppedInfo {
	return o.stopped
}

func (o *deprecatedHttpServer) Execute(command string) (string, error) {
	return o.options.HttpApi.Exec(command)
}

// Deprecated: Use 'NewHttpServerApi()' instead.
func NewHttpServer(exePath string, options *HttpServerOptions) (HttpServer, error) {
	if options.HttpApi == nil {
		a, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", options.Port))
		if err != nil {
			return nil, err
		}

		options.HttpApi, err = NewHttpApi(a, &HttpApiOptions{
			Timeout: 5 * time.Second,
		})
		if err != nil {
			return nil, err
		}
	}

	return &deprecatedHttpServer{
		exePath: exePath,
		options: options,
		mutex:   &sync.Mutex{},
		state:   Stopped,
		stopped: make(chan StoppedInfo),
	}, nil
}
