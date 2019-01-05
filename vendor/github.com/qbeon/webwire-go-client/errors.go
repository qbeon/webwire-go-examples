package client

import "fmt"

// ErrMismatchSubProto represents an error indicating that the sub-protocols of
// the client and the server don't match
type ErrMismatchSubProto struct {
	ClientSubProto []byte
	ServerSubProto []byte
}

// Error implements the error interface
func (err ErrMismatchSubProto) Error() string {
	return fmt.Sprintf(
		"mismatching sub-protocols: (client: %s; server: %s)",
		err.ClientSubProto,
		err.ServerSubProto,
	)
}
