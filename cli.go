package radareutil

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
)

type cliApi struct {
	config *Radare2Config
	r2     *r2Proc
}

func (o *cliApi) Start() error {
	err := o.r2.Start(Cli)
	if err != nil {
		return err
	}

	// Read initial data per pipe example.
	_, err = bufio.NewReader(o.r2.stdout).ReadByte()
	if err != nil {
		return err
	}

	return nil
}

func (o *cliApi) Kill() {
	o.r2.Kill()
}

func (o *cliApi) Status() Status {
	return o.r2.Status()
}

func (o *cliApi) OnStopped() chan StoppedInfo {
	return o.r2.OnStopped()
}

func (o *cliApi) ExecuteToJson(c string, p interface{}) error {
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

func (o *cliApi) Execute(cmd string) (string, error) {
	_, err := fmt.Fprintln(o.r2.stdin, cmd)
	if err != nil {
		return "", err
	}

	output, err := bufio.NewReader(o.r2.stdout).ReadBytes('\x00')
	if err != nil {
		return "", err
	}

	if o.config.DoNotTrimOutput {
		return string(output), nil
	}

	return string(bytes.TrimSpace(bytes.TrimRight(output, "\n\x00"))), nil
}

func NewCliApi(config *Radare2Config) (Api, error) {
	r2, err := newR2Proc(config)
	if err != nil {
		return nil, err
	}

	return &cliApi{
		config: config,
		r2:     r2,
	}, nil
}
