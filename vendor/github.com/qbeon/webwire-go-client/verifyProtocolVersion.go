package client

import (
	"fmt"

	wwr "github.com/qbeon/webwire-go"
)

// verifyProtocolVersion returns true if the given version of the webwire
// protocol is acceptable for this client
func verifyProtocolVersion(major, minor byte) error {
	if major != 2 {
		return wwr.ErrIncompatibleProtocolVersion{
			RequiredVersion:  fmt.Sprintf("%d.%d", major, minor),
			SupportedVersion: "2.x",
		}
	}
	return nil
}
