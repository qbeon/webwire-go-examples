package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	wwr "github.com/qbeon/webwire-go"
	"github.com/qbeon/webwire-go-examples/chatroom/shared"
)

// ChatRoomServer implements the webwire.ServerImplementation interface
type ChatRoomServer struct {
	connected map[wwr.Connection]bool
	lock      sync.RWMutex
}

// NewChatRoomServer constructs a new
// webwire server implementation instance
func NewChatRoomServer() *ChatRoomServer {
	return &ChatRoomServer{
		make(map[wwr.Connection]bool),
		sync.RWMutex{},
	}
}

/****************************************************************\
	Message Broadcaster
\****************************************************************/

// broadcastMessage sends a message on behalf of the given user
// to all connected clients
func (srv *ChatRoomServer) broadcastMessage(name string, msg string) {
	// Marshal message
	encoded, err := json.Marshal(shared.ChatMessage{
		User: name,
		Msg:  msg,
	})
	if err != nil {
		panic(fmt.Errorf("Couldn't marshal chat message: %s", err))
	}

	// Send message as a signal to each connected client
	srv.lock.RLock()
	log.Printf("Broadcast message to %d clients", len(srv.connected))
	for client := range srv.connected {
		// Send message as signal
		if err := client.Signal(nil, wwr.Payload{
			Encoding: wwr.EncodingUtf8,
			Data:     encoded,
		}); err != nil {
			log.Printf(
				"WARNING: failed sending signal to client %s : %s",
				client.RemoteAddr(),
				err,
			)
		}
	}
	srv.lock.RUnlock()
}

/****************************************************************\
	Authentication Handler
\****************************************************************/

// onAuth handles incoming authentication requests.
// It parses and verifies the provided credentials
// and either rejects the authentication or confirms it eventually
// creating a session and returning the session key
func (srv *ChatRoomServer) handleAuth(
	_ context.Context,
	client wwr.Connection,
	message wwr.Message,
) (wwr.Payload, error) {
	credentialsText, err := message.PayloadUtf8()
	if err != nil {
		return wwr.Payload{}, wwr.ErrRequest{
			Code:    "DECODING_FAILURE",
			Message: fmt.Sprintf("Failed decoding message: %s", err),
		}
	}

	log.Printf("Client attempts authentication: %s", client.RemoteAddr())

	// Try to parse credentials
	var credentials shared.AuthenticationCredentials
	if err := json.Unmarshal(
		[]byte(credentialsText),
		&credentials,
	); err != nil {
		return wwr.Payload{}, fmt.Errorf("Failed parsing credentials: %s", err)
	}

	// Verify username
	password, userExists := userAccounts[credentials.Name]
	if !userExists {
		return wwr.Payload{}, wwr.ErrRequest{
			Code:    "INEXISTENT_USER",
			Message: fmt.Sprintf("No such user: '%s'", credentials.Name),
		}
	}

	// Verify password
	if password != credentials.Password {
		return wwr.Payload{}, wwr.ErrRequest{
			Code:    "WRONG_PASSWORD",
			Message: "Provided password is wrong",
		}
	}

	// Finally create a new session
	if err := client.CreateSession(&shared.SessionInfo{
		Username: credentials.Name,
	}); err != nil {
		return wwr.Payload{}, fmt.Errorf("Couldn't create session: %s", err)
	}

	log.Printf(
		"Created session for user %s (%s)",
		client.RemoteAddr(),
		credentials.Name,
	)

	// Reply to the request, use default binary encoding
	return wwr.Payload{
		Encoding: wwr.EncodingBinary,
		Data:     []byte(client.SessionKey()),
	}, nil
}

/****************************************************************\
	Message Handler
\****************************************************************/

func (srv *ChatRoomServer) handleMessage(
	_ context.Context,
	client wwr.Connection,
	message wwr.Message,
) (wwr.Payload, error) {
	msgStr, err := message.PayloadUtf8()
	if err != nil {
		log.Printf(
			"Received invalid message from %s, "+
				"couldn't convert payload to UTF8: %s",
			client.RemoteAddr(),
			err,
		)
		return wwr.Payload{}, nil
	}

	log.Printf(
		"Received message from %s: '%s' (%d, %s)",
		client.RemoteAddr(),
		msgStr,
		len(msgStr),
		message.PayloadEncoding().String(),
	)

	name := "Anonymous"
	// Try to read the name from the session
	if client.HasSession() {
		name = client.SessionInfo("username").(string)
	}

	srv.broadcastMessage(name, string(msgStr))

	return wwr.Payload{}, nil
}

/****************************************************************\
	Hook implementations
\****************************************************************/

// OnSignal implements the webwire.ServerImplementation interface
// Does nothing, not needed in this example
func (srv *ChatRoomServer) OnSignal(
	_ context.Context,
	_ wwr.Connection,
	_ wwr.Message,
) {
}

// OnRequest implements the webwire.ServerImplementation interface.
// Receives the message and dispatches it to the according handler
func (srv *ChatRoomServer) OnRequest(
	ctx context.Context,
	client wwr.Connection,
	message wwr.Message,
) (response wwr.Payload, err error) {
	switch string(message.Name()) {
	case "auth":
		return srv.handleAuth(ctx, client, message)
	case "msg":
		return srv.handleMessage(ctx, client, message)
	}
	return wwr.Payload{}, wwr.ErrRequest{
		Code:    "BAD_REQUEST",
		Message: fmt.Sprintf("Unsupported request name: %s", message.Name()),
	}
}

// OnClientConnected implements the webwire.ServerImplementation interface.
// Registers new connected clients
func (srv *ChatRoomServer) OnClientConnected(
	connOpts wwr.ConnectionOptions,
	newClient wwr.Connection,
) {
	log.Printf(
		"New client connected: %s | %s",
		newClient.RemoteAddr(),
		connOpts.Info[0].([]byte),
	)
	srv.lock.Lock()
	srv.connected[newClient] = true
	srv.lock.Unlock()
}

// OnClientDisconnected implements the webwire.ServerImplementation interface.
// Deregisters gone clients
func (srv *ChatRoomServer) OnClientDisconnected(
	client wwr.Connection,
	reason error,
) {
	log.Printf(
		"Client %s disconnected, reason: %s",
		client.RemoteAddr(),
		reason,
	)
	srv.lock.Lock()
	delete(srv.connected, client)
	srv.lock.Unlock()
}
