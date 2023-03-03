package sdk

import (
	"strings"
)

// ListError is a wrapper to slice of `error`.
type ListError struct{ Errors []error }

func (e *ListError) Unwrap() error {
	_ = e.Err()

	return e.Pop(len(e.Errors) - 1)
}

// Error is a method that satisfied an error interface.
func (e *ListError) Error() string {
	_ = e.Err()

	v := []string{}
	for i := 0; i < len(e.Errors); i++ {
		v = append(v, e.Errors[i].Error())
	}

	return strings.Join(v, "\n")
}

// Pop will remove the errors from the given index, use i = -1 to remove from
// last index.
func (e *ListError) Pop(i int) error {
	if i == -1 && len(e.Errors) > 0 {
		i = len(e.Errors) - 1
	} else if i < 0 || i >= len(e.Errors) {
		return nil
	}

	err := e.Errors[i]
	e.Errors = append(e.Errors[:i], e.Errors[i+1:]...)

	return err
}

// Add will append the given error to the list.
func (e *ListError) Add(errs ...error) *ListError {
	for i := range errs {
		if errs[i] != nil {
			e.Errors = append(e.Errors, errs[i])
		}
	}

	return e
}

// Err will validate & remove any nil error in the list.
func (e *ListError) Err() error {
	for i, err := range e.Errors {
		if err == nil {
			_ = e.Pop(i)
		}
	}

	if len(e.Errors) < 1 {
		return nil
	}

	return e
}
