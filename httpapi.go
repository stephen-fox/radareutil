package radareutil

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
