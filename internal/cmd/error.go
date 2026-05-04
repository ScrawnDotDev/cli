package cmd

import "errors"

var (
	ErrMissingCommand = errors.New("missing command")
	ErrUnknownCommand = errors.New("unknown command")
)

type CommandError struct {
	Summary string
	Detail  string
}

func (e *CommandError) Error() string {
	if e.Detail == "" {
		return e.Summary
	}
	return e.Summary + ": " + e.Detail
}