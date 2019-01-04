package client

import (
	"encoding/json"
	"fmt"

	"github.com/qbeon/webwire-go"
	wwr "github.com/qbeon/webwire-go"
	"github.com/qbeon/webwire-go/message"
	"github.com/qbeon/webwire-go/payload"
)

func (clt *client) handleSessionCreated(msgPayload payload.Payload) {
	var encoded webwire.JSONEncodedSession
	if err := json.Unmarshal(msgPayload.Data, &encoded); err != nil {
		clt.options.ErrorLog.Printf(
			"Failed unmarshalling session object: %s",
			err,
		)
		return
	}

	// parse attached session info
	var parsedSessInfo webwire.SessionInfo
	if encoded.Info != nil && clt.options.SessionInfoParser != nil {
		parsedSessInfo = clt.options.SessionInfoParser(encoded.Info)
	}

	clt.sessionLock.Lock()
	clt.session = &webwire.Session{
		Key:      encoded.Key,
		Creation: encoded.Creation,
		Info:     parsedSessInfo,
	}
	clt.sessionLock.Unlock()
	clt.impl.OnSessionCreated(clt.session)
}

func (clt *client) handleSessionClosed() {
	// Destroy local session
	clt.sessionLock.Lock()
	clt.session = nil
	clt.sessionLock.Unlock()

	clt.impl.OnSessionClosed()
}

func (clt *client) handleInternalError(reqIdent [8]byte) {
	// Fail request
	clt.requestManager.Fail(reqIdent, wwr.ErrInternal{})
}

func (clt *client) handleReplyShutdown(reqIdent [8]byte) {
	clt.requestManager.Fail(reqIdent, wwr.ErrServerShutdown{})
}

func (clt *client) handleSessionNotFound(reqIdent [8]byte) {
	clt.requestManager.Fail(reqIdent, wwr.ErrSessionNotFound{})
}

func (clt *client) handleMaxSessConnsReached(reqIdent [8]byte) {
	clt.requestManager.Fail(reqIdent, wwr.ErrMaxSessConnsReached{})
}

func (clt *client) handleSessionsDisabled(reqIdent [8]byte) {
	clt.requestManager.Fail(reqIdent, wwr.ErrSessionsDisabled{})
}

// handleMessage handles incoming messages
func (clt *client) handleMessage(msg *message.Message) (err error) {
	// Recover user-space panics to avoid leaking memory through unreleased
	// message buffer
	defer func() {
		if recvErr := recover(); recvErr != nil {
			r, ok := recvErr.(error)
			if !ok {
				err = fmt.Errorf("unexpected panic: %v", recvErr)
			} else {
				err = r
			}
		}
	}()

	switch msg.MsgType {
	case message.MsgReplyBinary:
		clt.requestManager.Fulfill(msg)
		// Don't release the buffer, make the user responsible for releasing it
	case message.MsgReplyUtf8:
		clt.requestManager.Fulfill(msg)
		// Don't release the buffer, make the user responsible for releasing it
	case message.MsgReplyUtf16:
		clt.requestManager.Fulfill(msg)
		// Don't release the buffer, make the user responsible for releasing it

	case message.MsgReplyShutdown:
		clt.handleReplyShutdown(msg.MsgIdentifier)
		// Release the buffer
		msg.Close()
	case message.MsgReplySessionNotFound:
		clt.handleSessionNotFound(msg.MsgIdentifier)
		// Release the buffer
		msg.Close()
	case message.MsgReplyMaxSessConnsReached:
		clt.handleMaxSessConnsReached(msg.MsgIdentifier)
		// Release the buffer
		msg.Close()
	case message.MsgReplySessionsDisabled:
		clt.handleSessionsDisabled(msg.MsgIdentifier)
		// Release the buffer
		msg.Close()
	case message.MsgReplyError:
		// The message name contains the error code in case of
		// error reply messages, while the UTF8 encoded error message is
		// contained in the message payload
		clt.requestManager.Fail(msg.MsgIdentifier, wwr.ErrRequest{
			Code:    string(msg.MsgName),
			Message: string(msg.MsgPayload.Data),
		})
		// Release the buffer
		msg.Close()
	case message.MsgReplyInternalError:
		clt.handleInternalError(msg.MsgIdentifier)
		// Release the buffer
		msg.Close()

	case message.MsgSignalBinary:
		fallthrough
	case message.MsgSignalUtf8:
		fallthrough
	case message.MsgSignalUtf16:
		clt.impl.OnSignal(msg)
		// Realease the buffer after the OnSignal user-space hook is executed
		// because it's referenced there through the payload
		msg.Close()

	case message.MsgNotifySessionCreated:
		clt.handleSessionCreated(msg.MsgPayload)
		// Release the buffer after the OnSessionCreated user-space hook is
		// executed because it's referenced there through the payload
		msg.Close()
	case message.MsgNotifySessionClosed:
		// Release the buffer before calling the OnSessionClosed user-space hook
		msg.Close()
		clt.handleSessionClosed()
	default:
		// Release the buffer
		msg.Close()
		clt.options.WarnLog.Printf(
			"Strange message type received: '%d'\n",
			msg.MsgType,
		)
	}
	return nil
}
