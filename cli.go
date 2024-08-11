package radareutil

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type cliApi struct {
	config *Radare2Config
	r2     *r2Proc
}

func (o *cliApi) Start() error {
	err := o.r2.start(Cli)
	if err != nil {
		return err
	}

	// Read initial data per pipe example.
	_, err = o.r2.stdout.ReadBytes(0x00)
	if err != nil {
		return err
	}

	return nil
}

func (o *cliApi) Interrupt() error {
	return o.r2.interrupt()
}

func (o *cliApi) Kill() {
	o.r2.kill()
}

func (o *cliApi) Status() Status {
	return o.r2.status()
}

func (o *cliApi) OnStopped() chan StoppedInfo {
	return o.r2.onStopped()
}

func (o *cliApi) ExecuteToJson(c string, p interface{}) error {
	output, err := o.ExecuteToBytes(c)
	if err != nil {
		return err
	}

	err = json.Unmarshal(output, p)
	if err != nil {
		return err
	}

	return nil
}

func (o *cliApi) Execute(cmd string) (string, error) {
	raw, err := o.ExecuteToBytes(cmd)
	if err != nil {
		return string(raw), err
	}

	return string(raw), nil
}

func (o *cliApi) ExecuteToBytes(cmd string) ([]byte, error) {
	current := o.r2.status().State
	if current != Running {
		return nil, fmt.Errorf("cannot execute command - state is %s", current)
	}

	_, err := fmt.Fprintln(o.r2.stdin, cmd)
	if err != nil {
		return nil, err
	}

	raw, err := o.r2.stdout.ReadBytes(0x00)
	if err != nil {
		return nil, err
	}

	if o.config.DoNotTrimOutput {
		return raw, nil
	}

	return bytes.TrimRight(raw, "\n\x00"), nil
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
