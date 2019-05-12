package radareutil

type State string

const (
	Stopped State = "stopped"
	Dead    State = "dead"
	Running State = "running"
)

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
