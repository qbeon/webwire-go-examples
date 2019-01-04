package client

import (
	"time"

	wwr "github.com/qbeon/webwire-go"
)

func (clt *client) backgroundReconnect() {
	clt.connectingLock.Lock()
	if clt.connecting {
		clt.connectingLock.Unlock()
		return
	}
	clt.connecting = true
	clt.connectingLock.Unlock()
	go func() {
		for {
			err := clt.connect()
			switch err := err.(type) {
			case nil:
				clt.connectingLock.Lock()
				clt.backReconn.flush(nil)
				clt.connecting = false
				clt.connectingLock.Unlock()
				return
			case wwr.ErrDisconnected:
				time.Sleep(clt.options.ReconnectionInterval)
			default:
				// Unexpected error
				clt.backReconn.flush(err)
				return
			}
		}
	}()
}
