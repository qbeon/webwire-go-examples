package client

import "bytes"

func copyBytes(source []byte) []byte {
	if source == nil {
		return nil
	}
	result := make([]byte, len(source))
	copy(result, source)
	return result
}

// compareSubProtocols compares the given sub-protocol names and returns a
// mismatch error if they don't match, otherwise returns nil
func compareSubProtocols(serverSubProto, clientSubProto []byte) error {
	if serverSubProto != nil && clientSubProto == nil {
		return ErrMismatchSubProto{
			ServerSubProto: copyBytes(serverSubProto),
			ClientSubProto: nil,
		}
	} else if serverSubProto == nil && clientSubProto != nil {
		return ErrMismatchSubProto{
			ServerSubProto: nil,
			ClientSubProto: copyBytes(clientSubProto),
		}
	} else if serverSubProto != nil &&
		clientSubProto != nil && !bytes.Equal(
		serverSubProto,
		clientSubProto,
	) {
		return ErrMismatchSubProto{
			ServerSubProto: copyBytes(serverSubProto),
			ClientSubProto: copyBytes(clientSubProto),
		}
	}
	return nil
}
