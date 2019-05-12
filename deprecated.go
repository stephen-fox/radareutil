package radareutil

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
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

	return &defaultHttpServer{
		exePath: finalExePath,
		options: options,
		mutex:   &sync.Mutex{},
		state:   Stopped,
		stopped: make(chan StoppedInfo),
	}, nil
}
