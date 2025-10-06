package utils

// TODO: determine if this is useful
type Step struct {
	Name       string
	Action     func() error
	Compensate func() error
}

type Saga []Step
