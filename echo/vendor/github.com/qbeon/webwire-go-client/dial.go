package client

import (
	"bytes"
	"fmt"
	"sync/atomic"
	"time"

	wwr "github.com/qbeon/webwire-go"
	"github.com/qbeon/webwire-go/message"
)

// dial tries to dial in the server and await an approval including the endpoint
// metadata before the configured dialing timeout is reached.
// clt.dial should only be called from within clt.connect.
func (clt *client) dial() (srvConf message.ServerConfiguration, err error) {
	deadline := time.Now().Add(clt.options.DialingTimeout)
	clt.dialingTimer.Reset(clt.options.DialingTimeout)

	type dialResult struct {
		serverConfiguration message.ServerConfiguration
		err                 error
	}

	result := make(chan dialResult, 1)
	abortAwait := uint32(0)

	go func() {
		// Dial
		if err := clt.conn.Dial(deadline); err != nil {
			result <- dialResult{err: err}
			return
		}

		// Abort if timed out
		if atomic.LoadUint32(&abortAwait) > 0 {
			clt.conn.Close()
			return
		}

		// Get a message buffer from the pool
		msg := clt.messagePool.Get()

		failureCleanup := func() {
			clt.conn.Close()
			msg.Close()
		}

		// Abort if timed out
		if atomic.LoadUint32(&abortAwait) > 0 {
			failureCleanup()
			return
		}

		// Await the server accept-conf response
		if err := clt.conn.Read(msg, deadline); err != nil {
			if err.IsCloseErr() {
				// Regular connection closure
				result <- dialResult{err: wwr.ErrDisconnected{
					Cause: fmt.Errorf(
						"couldn't read accept-conf message during dial: %s",
						err,
					),
				}}
			} else {
				// Error during reading of server accept-conf message
				result <- dialResult{err: fmt.Errorf(
					"read err: %s",
					err.Error(),
				)}
			}
			failureCleanup()
			return
		}

		// Abort if timed out
		if atomic.LoadUint32(&abortAwait) > 0 {
			failureCleanup()
			return
		}

		if msg.MsgType != message.MsgAcceptConf {
			result <- dialResult{err: fmt.Errorf(
				"unexpected message type: %d (expected accept-conf message)",
				msg.MsgType,
			)}
			failureCleanup()
			return
		}

		// Verify the protocol version
		if err := verifyProtocolVersion(
			msg.ServerConfiguration.MajorProtocolVersion,
			msg.ServerConfiguration.MinorProtocolVersion,
		); err != nil {
			result <- dialResult{err: err}
			failureCleanup()
			return
		}

		// Ensure sub-protocols match
		if msg.ServerConfiguration.SubProtocolName != nil &&
			clt.options.SubProtocolName == nil {
			result <- dialResult{err: fmt.Errorf(
				"mismatching sub-protocols (server: %s; client: nil)",
				msg.ServerConfiguration.SubProtocolName,
			)}
			failureCleanup()
			return
		} else if msg.ServerConfiguration.SubProtocolName == nil &&
			clt.options.SubProtocolName != nil {
			result <- dialResult{err: fmt.Errorf(
				"mismatching sub-protocols (server: nil; client: %s)",
				clt.options.SubProtocolName,
			)}
			failureCleanup()
			return
		} else if msg.ServerConfiguration.SubProtocolName != nil &&
			clt.options.SubProtocolName != nil && !bytes.Equal(
			msg.ServerConfiguration.SubProtocolName,
			clt.options.SubProtocolName,
		) {
			result <- dialResult{err: fmt.Errorf(
				"mismatching sub-protocols (server: %s; client: %s)",
				msg.ServerConfiguration.SubProtocolName,
				clt.options.SubProtocolName,
			)}
			failureCleanup()
			return
		}

		// Ensure the message buffer size is similar to the one on the server
		if clt.options.MessageBufferSize !=
			msg.ServerConfiguration.MessageBufferSize {
			result <- dialResult{err: fmt.Errorf(
				"mismatching message buffer capacity (server: %d; client: %d)",
				msg.ServerConfiguration.MessageBufferSize,
				clt.options.MessageBufferSize,
			)}
			failureCleanup()
			return
		}

		// Finish successful dial
		result <- dialResult{
			serverConfiguration: msg.ServerConfiguration,
		}
		msg.Close()
	}()

	select {
	case <-clt.dialingTimer.C:
		// Abort due to timeout
		atomic.StoreUint32(&abortAwait, 1)
		err = wwr.ErrDisconnected{}

	case result := <-result:
		srvConf = result.serverConfiguration
		err = result.err
	}

	clt.dialingTimer.Stop()

	return
}
