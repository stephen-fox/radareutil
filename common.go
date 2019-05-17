package radareutil

import (
	"fmt"
)

type Mode string

func (o Mode) String() string {
	return string(o)
}

const (
	Unset Mode = ""
	Cli   Mode = "cli"
	Http  Mode = "http"
)

type State string

func (o State) String() string {
	return string(o)
}

const (
	Stopped State = "stopped"
	Dead    State = "dead"
	Running State = "running"
)

type Api interface {
	Start() error
	Kill()
	OnStopped() chan StoppedInfo
	Status() Status
	Execute(command string) (string, error)
	ExecuteToJson(command string, pointer interface{}) error
	ExecuteToBytes(command string) ([]byte, error)
}

type Status struct {
	State State
}

type StoppedInfo struct {
	err error
	out string
}

func (o *StoppedInfo) Err() error {
	return o.err
}

func (o *StoppedInfo) CombinedOutput() string {
	return o.out
}

type Radare2Config struct {
	ExecutablePath     string
	CustomCliArgs      []string
	DoNotTrimOutput    bool
	SaveOutput         bool
	DebugPid           int
	DisableHttpSandbox bool
	HttpPort           int
	DetachOnStop       bool
}

func (o *Radare2Config) Validate() error {
	exePathFinal, err := fullyQualifiedBinaryPath(o.ExecutablePath)
	if err != nil {
		return err
	}

	o.ExecutablePath = exePathFinal

	return nil
}

func (o *Radare2Config) Args(mode Mode) ([]string, error) {
	if o.CustomCliArgs != nil {
		return o.CustomCliArgs, nil
	}

	var args []string

	switch mode {
	case Cli:
		args = append(args, "-q")
		args = append(args, "-0")
	case Http:
		if o.HttpPort > 0 {
			args = append(args, fmt.Sprintf("%s%d", httpServerArg, o.HttpPort))
		} else {
			args = append(args, httpServerArg)
		}

		if o.DisableHttpSandbox {
			args = append(args, "-e", "http.sandbox=false")
		}
	default:
		return nil, fmt.Errorf("unknown mode '%s'", mode.String())
	}

	if o.DebugPid > 0 {
		args = append(args, "-d", fmt.Sprintf("%d", o.DebugPid))
	} else {
		args = append(args, "--")
	}

	return args, nil
}
