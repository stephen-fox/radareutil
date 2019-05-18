package radareutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	httpServerArg = "-c=h"
)

type httpServerApi struct {
	config  *Radare2Config
	client  *http.Client
	address *url.URL
	r2      *r2Proc
}

func (o *httpServerApi) Start() error {
	return o.r2.Start(Http)
}

func (o *httpServerApi) Interrupt() error {
	return o.r2.interrupt()
}

func (o *httpServerApi) Kill() {
	o.r2.Kill()
}

func (o *httpServerApi) Status() Status {
	return o.r2.Status()
}

func (o *httpServerApi) OnStopped() chan StoppedInfo {
	return o.r2.OnStopped()
}

func (o *httpServerApi) ExecuteToJson(c string, p interface{}) error {
	output, err := o.Execute(c)
	if err != nil {
		return err
	}

	err = json.Unmarshal([]byte(output), p)
	if err != nil {
		return err
	}

	return nil
}

func (o *httpServerApi) Execute(command string) (string, error) {
	result, err := o.ExecuteToBytes(command)
	if err != nil {
		return string(result), err
	}

	return string(result), nil
}

func (o *httpServerApi) ExecuteToBytes(command string) ([]byte, error) {
	current := o.r2.Status().State
	if current != Running {
		return nil, fmt.Errorf("cannot execute command - state is %s", current)
	}

	result, err := executeHttpCall(command, o.address, o.client, !o.config.DoNotTrimOutput)
	if err != nil {
		return result, err
	}

	return result, nil
}

// NewHttpServerApi returns a new instance of radare2 running in HTTP
// server mode.
//
// WARNING: This insecure - use at your own risk!
//
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
func NewHttpServerApi(config *Radare2Config) (Api, error) {
	a, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", config.HttpPort))
	if err != nil {
		return nil, err
	}

	r2, err := newR2Proc(config)
	if err != nil {
		return nil, err
	}

	return &httpServerApi{
		config:  config,
		r2:      r2,
		client:  &http.Client{
			Timeout: 5 * time.Second,
		},
		address: a,
	}, nil
}

func NewCustomHttpServerApi(address *url.URL, httpClient *http.Client, config *Radare2Config) (Api, error) {
	r2, err := newR2Proc(config)
	if err != nil {
		return nil, err
	}

	return &httpServerApi{
		config:  config,
		r2:      r2,
		client:  httpClient,
		address: address,
	}, nil
}

func executeHttpCall(command string, address *url.URL, httpClient *http.Client, trim bool) ([]byte, error) {
	resp, err := httpClient.Get(address.String() + cmdSubPath + "/" + command)
	if err != nil {
		return nil, err
	}

	if resp.Body == nil {
		return nil, errors.New("http response body is empty")
	}
	defer resp.Body.Close()

	raw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		base := "request failed with code " + strconv.Itoa(resp.StatusCode)

		if len(raw) == 0 {
			return nil, errors.New(base)
		}

		return nil, errors.New(base + " - details - " + string(raw))
	}

	if trim {
		raw = bytes.TrimSpace(raw)
	}

	return raw, nil
}
