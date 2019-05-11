package radareutil

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	httpServerArg = "-c=h"
)

type State string

const (
	Stopped State = "stopped"
	Dead    State = "dead"
	Running State = "running"
)

type HttpServerOptions struct {
	OnStop         chan ServerStoppedInfo
	DisableSandbox bool
	DebugPid       int
	Port           int
	// DetachOnStop requires that HttpApi be set.
	DetachOnStop   bool
	HttpApi        HttpApi
}

type ServerStoppedInfo struct {
	err error
	out string
}

func (o *ServerStoppedInfo) Err() error {
	return o.err
}

func (o *ServerStoppedInfo) CombinedOutput() string {
	return o.out
}

type HttpServer interface {
	Start() error
	Stop()
	Restart() error
	Options() *HttpServerOptions
	Status() ServerStatus
}

type defaultHttpServer struct {
	exePath string
	mutex   *sync.Mutex
	server  *exec.Cmd
	options *HttpServerOptions
	state   State
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

	var output *bytes.Buffer
	if o.options.OnStop != nil {
		output = bytes.NewBuffer(nil)
		radare.Stderr = output
		radare.Stdout = output
	}

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

	if o.state != Stopped {
		o.state = Dead
	}

	if o.options.OnStop != nil {
		var info ServerStoppedInfo
		if o.state == Dead {
			info.err = err
		}
		if output != nil {
			info.out = output.String()
		}
		o.options.OnStop <- info
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

func (o *defaultHttpServer) Status() ServerStatus {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	return ServerStatus{
		State: o.state,
	}
}

type ServerStatus struct {
	State State
}

// NewHttpServer returns a new instance of radare2 running in HTTP server mode.
// Sadly, there is not really any documentation about this feature. Here is a
// good primer by Megabeets describing how this works:
// (https://reverseengineering.stackexchange.com/a/18345)
//
//	radare2 comes with its own webserver. Although at first, it might seems
//	like an overkill, its actually quite useful, especially when you want
//	to debug embedded systems, or simply to execute commands from a remote
//	terminal.
//
//	Simply launch the web server with =h <port> and connect to it with any
//	HTTP client.
//
//	You can print the help for this command by using =h?:
//
//	[0x00000000]> =h?
//	|Usage:  =[hH] [...] # http server
//	| http server:
//	| =h port       listen for http connections (r2 -qc=H /bin/ls)
//	| =h-           stop background webserver
//	| =h--          stop foreground webserver
//	| =h*           restart current webserver
//	| =h& port      start http server in background
//	| =H port       launch browser and listen for http
//	| =H& port      launch browser and listen for http in background
//
//	So let's use a oneliner command to spawn a radare2 web server with a
//	session to our beloved /bin/ls/:
//
//	$ r2 -c=h /bin/ls
//	Starting http server...
//	open http://localhost:9090/
//	r2 -C http://localhost:9090/cmd/
//
//	Good, now that we have an HTTP server running with an open session,
//	let's connect to it.
//
//	You can do this with curl:
//
//	$ curl http://127.0.0.1:9090/cmd/?EHello,World!
//	.--.     .--------------.
//	| _|     |              |
//	| O O   <  Hello,World! |
//	|  |  |  |              |
//	|| | /   `--------------'
//	|`-'|
//	`---'
func NewHttpServer(exePath string, options *HttpServerOptions) (HttpServer, error) {
	if !filepath.IsAbs(exePath) && !strings.ContainsAny("\\/", exePath) {
		whereOutputRaw, err := exec.Command("where", exePath).CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("failed to lookup radare binary - %s - output: '%s'", err.Error(), whereOutputRaw)
		}

		exePath = string(bytes.TrimSpace(whereOutputRaw))

		_, err = os.Stat(exePath)
		if err != nil {
			return nil, err
		}
	}

	return &defaultHttpServer{
		exePath: exePath,
		options: options,
		mutex:   &sync.Mutex{},
		state:   Stopped,
	}, nil
}
