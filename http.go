package radareutil

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"os/exec"
	"path"
	"strconv"
	"time"
)

const (
	httpServerArg = "-c=h"

	cmdSubPath = "/cmd"
)

type HttpServerOptions struct {
	DisableSandbox  bool
	InitialDebugPid int
	Port            int
}

type HttpApi interface {
	Exec(command string) (string, error)
}

type defaultHttpApi struct {
	httpClient *http.Client
	address    *url.URL
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
	
	return content, nil
}

func NewHttpApi(address *url.URL) (HttpApi, error) {
	return defaultHttpApi{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		address: address,
	}, nil
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
func NewHttpServer(exePath string, options HttpServerOptions) (*exec.Cmd, error) {
	var args []string

	if options.Port > 0 {
		args = append(args, httpServerArg + strconv.Itoa(options.Port))
	} else {
		args = append(args, httpServerArg)
	}

	if options.DisableSandbox {
		args = append(args, "-e", "http.sandbox=false")
	}

	if options.InitialDebugPid > 0 {
		args = append(args, "-d", strconv.Itoa(options.InitialDebugPid))
	} else {
		args = append(args, "--")
	}

	r := exec.Command(path.Base(exePath), args...)
	r.Dir = path.Dir(exePath)

	err := r.Start()
	if err != nil {
		return &exec.Cmd{}, err
	}

	return r, nil
}
