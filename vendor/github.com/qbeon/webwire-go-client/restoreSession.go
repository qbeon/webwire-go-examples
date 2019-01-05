package client

import (
	"context"
	"encoding/json"
	"fmt"

	webwire "github.com/qbeon/webwire-go"
	"github.com/qbeon/webwire-go/message"
	"github.com/qbeon/webwire-go/payload"
)

// RestoreSession tries to restore the previously opened session.
// Fails if a session is currently already active
func (clt *client) RestoreSession(
	ctx context.Context,
	sessionKey []byte,
) error {
	if ctx == nil {
		ctx = context.Background()
	}

	// Apply exclusive lock
	clt.apiLock.Lock()

	// Set default deadline if no deadline is yet specified
	closeCtx := func() {}
	_, deadlineWasSet := ctx.Deadline()
	if !deadlineWasSet {
		ctx, closeCtx = context.WithTimeout(
			ctx,
			clt.options.DefaultRequestTimeout,
		)
	}

	clt.sessionLock.RLock()
	if clt.session != nil {
		clt.apiLock.Unlock()
		clt.sessionLock.RUnlock()
		closeCtx()
		return fmt.Errorf(
			"Can't restore session if another one is already active",
		)
	}
	clt.sessionLock.RUnlock()

	if err := clt.tryAutoconnect(ctx, deadlineWasSet); err != nil {
		clt.apiLock.Unlock()
		closeCtx()
		return err
	}
	if err := clt.restoreSession(ctx, sessionKey); err != nil {
		clt.apiLock.Unlock()
		closeCtx()
		return err
	}

	clt.apiLock.Unlock()
	closeCtx()
	return nil
}

// restoreSession sends a session restoration request
// and decodes the session object from the received reply.
// Expects the client to be connected beforehand
func (clt *client) restoreSession(
	ctx context.Context,
	sessionKey []byte,
) error {
	reply, err := clt.sendNamelessRequest(
		ctx,
		message.MsgRequestRestoreSession,
		payload.Payload{
			Encoding: webwire.EncodingBinary,
			Data:     sessionKey,
		},
	)
	if err != nil {
		return err
	}

	// Unmarshal JSON encoded session object
	var encodedSessionObj webwire.JSONEncodedSession
	if err := json.Unmarshal(
		reply.Payload(),
		&encodedSessionObj,
	); err != nil {
		reply.Close()
		return fmt.Errorf(
			"couldn't unmarshal restored session from reply: %s",
			err,
		)
	}

	reply.Close()

	// Parse session info object
	var decodedInfo webwire.SessionInfo
	if encodedSessionObj.Info != nil {
		decodedInfo = clt.options.SessionInfoParser(encodedSessionObj.Info)
	}

	newSession := &webwire.Session{
		Key:      encodedSessionObj.Key,
		Creation: encodedSessionObj.Creation,
		Info:     decodedInfo,
	}

	clt.impl.OnSessionCreated(newSession)

	clt.sessionLock.Lock()
	clt.session = newSession
	clt.sessionLock.Unlock()

	return nil
}
