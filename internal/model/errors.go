package model

import "fmt"

// DomainError wraps a domain rule violation with a human readable message.
type DomainError struct {
	Op  string
	Err error
}

func (e *DomainError) Error() string {
	return fmt.Sprintf("model.%s: %v", e.Op, e.Err)
}

func (e *DomainError) Unwrap() error { return e.Err }

// NewDomainError builds a DomainError.
func NewDomainError(op string, err error) *DomainError {
	return &DomainError{Op: op, Err: err}
}
