package radareutil

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
	return o.r2.Start()
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

func (o *httpServerApi) Execute(command string) (string, error) {
	current := o.r2.Status().State
	if current != Running {
		return "", fmt.Errorf("cannot execute command - state is %s", current)
	}

	result, err := executeHttpCall(command, o.address, o.client)
	if err != nil {
		return result, err
	}

	if !o.config.DoNotTrimOutput {
		result = strings.TrimSpace(result)
	}

	return result, nil
}

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

func executeHttpCall(command string, address *url.URL, httpClient *http.Client) (string, error) {
	resp, err := httpClient.Get(address.String() + cmdSubPath + "/" + command)
	if err != nil {
		return "", err
	}

	if resp.Body == nil {
		return "", errors.New("http response body is empty")
	}
	defer resp.Body.Close()

	raw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	content := string(raw)

	if resp.StatusCode != http.StatusOK {
		base := "request failed with code " + strconv.Itoa(resp.StatusCode)

		if len(content) == 0 {
			return "", errors.New(base)
		}

		return "", errors.New(base + " - details - " + content)
	}

	return content, nil
}
