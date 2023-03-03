package sdk

import (
	"errors"
)

var (
	ErrNoResult      = errors.New("No result")
	ErrAlreadyClosed = errors.New("Already closed")

	ErrOverflow     = errors.New("Overflow")
	ErrInvalidValue = errors.New("Invalid value")

	ErrTracerServiceNameRequired = errors.New("tracer: service name required")
	ErrTracerEndpointRequired    = errors.New("tracer: endpoint required")
)

func PanicIf(cond bool, v interface{}) {
	if cond {
		panic(v)
	}
}
