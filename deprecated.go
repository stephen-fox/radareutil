package radareutil

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	cmdSubPath = "/cmd"
)

type HttpApiOptions struct {
	Timeout             time.Duration
	DoNotTrimWhiteSpace bool
}

type HttpApi interface {
	Exec(command string) (string, error)
}

type defaultHttpApi struct {
	httpClient *http.Client
	address    *url.URL
	options    *HttpApiOptions
}

func (o defaultHttpApi) Exec(command string) (string, error) {
	resp, err := o.httpClient.Get(o.address.String() + cmdSubPath + "/" + command)
	if err != nil {
		return "", err
	}

	if resp.Body == nil {
		return "", errors.New("No body")
	}
	defer resp.Body.Close()

	raw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	content := string(raw)

	if resp.StatusCode != http.StatusOK {
		base := "Request failed with code " + strconv.Itoa(resp.StatusCode)

		if len(content) == 0 {
			return "", errors.New(base)
		}

		return "", errors.New(base + ". Details - " + content)
	}

	if !o.options.DoNotTrimWhiteSpace {
		content = strings.TrimSpace(content)
	}

	return content, nil
}

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

type HttpServer interface {
	Options() *HttpServerOptions
	Start() error
	Stop()
	OnStopped() chan StoppedInfo
	Restart() error
	Status() Status
	Execute(command string) (string, error)
}

type HttpServerOptions struct {
	DisableSandbox bool
	DebugPid       int
	Port           int
	// DetachOnStop requires that HttpApi be set.
	DetachOnStop   bool
	HttpApi        HttpApi
}

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
	finalExePath, err := fullyQualifiedBinaryPath(exePath)
	if err != nil {
		return nil, err
	}

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
		exePath: finalExePath,
		options: options,
		mutex:   &sync.Mutex{},
		state:   Stopped,
		stopped: make(chan StoppedInfo),
	}, nil
}
