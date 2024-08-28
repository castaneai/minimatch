package statestore

import "errors"

var (
	ErrTicketNotFound     = errors.New("ticket not found")
	ErrAssignmentNotFound = errors.New("assignment not found")
)
