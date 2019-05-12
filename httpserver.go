package radareutil

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
)

const (
	httpServerArg = "-c=h"
)

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
