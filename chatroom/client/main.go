package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
	wwrclt "github.com/qbeon/webwire-go-client"
	"github.com/qbeon/webwire-go-examples/chatroom/shared"
	wwrgorilla "github.com/qbeon/webwire-go-gorilla"
)

// loadWebwireCACertificate loads the webwire CA certificate from a file
// to make the client accept the self-signed TLS certificate
func loadWebwireCACertificate(
	certFilePath string,
	dialer *websocket.Dialer,
) error {
	if dialer.TLSClientConfig.ClientCAs == nil {
		dialer.TLSClientConfig.ClientCAs = x509.NewCertPool()
	}

	fileContents, err := ioutil.ReadFile(certFilePath)
	if err != nil {
		return fmt.Errorf("couldn't read webwire CA certificate file: %s", err)
	}

	block, _ := pem.Decode(fileContents)
	rootCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("couldn't parse webwire CA x509 certificate: %s", err)
	}

	dialer.TLSClientConfig.ClientCAs.AddCert(rootCert)

	return nil
}

// ChatroomClient implements the wwrclt.Implementation interface
type ChatroomClient struct {
	connection wwrclt.Client
}

// NewChatroomClient constructs and returns a new chatroom client instance
func NewChatroomClient(serverAddr url.URL) (*ChatroomClient, error) {
	newChatroomClient := &ChatroomClient{}

	// Initialize dialer
	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	/*
		Load webwire CA certificate. You can remove this call if the webwire CA
		certificate is installed on your system
	*/
	if err := loadWebwireCACertificate(
		"../server/wwrexampleCA.pem",
		&dialer,
	); err != nil {
		return nil, err
	}

	// Initialize connection
	connection, err := wwrclt.NewClient(
		newChatroomClient,
		wwrclt.Options{
			// Default timeout for timed requests
			DefaultRequestTimeout: 10 * time.Second,

			// Retry connection establishment after 2 seconds on failure
			ReconnectionInterval: 2 * time.Second,

			// Session info parser function must override the default one
			// for the session info object to be typed as shared.SessionInfo
			SessionInfoParser: shared.SessionInfoParser,

			// Custom loggers
			WarnLog: log.New(
				os.Stdout,
				"WARN: ",
				log.Ldate|log.Ltime|log.Lshortfile,
			),
			ErrorLog: log.New(
				os.Stderr,
				"ERR: ",
				log.Ldate|log.Ltime|log.Lshortfile,
			),

			// Define the sub-protocol name to be able to connect to the server
			SubProtocolName: []byte("chatroom-example-protocol"),
		},
		&wwrgorilla.ClientTransport{
			ServerAddress: serverAddr,
			Dialer:        dialer,
		},
	)
	if err != nil {
		return nil, err
	}

	newChatroomClient.connection = connection

	return newChatroomClient, nil
}

var serverAddr = flag.String("addr", "localhost:9090", "server address")
var password = flag.String("pass", "", "password")
var username = flag.String("name", "", "username")

func main() {
	// Parse command line arguments
	flag.Parse()

	// Create a new chatroom client instance including its connection
	serverAddr := url.URL{
		Scheme: "https",
		Host:   *serverAddr,
		Path:   "/",
	}

	// Initialize client
	chatroomClient, err := NewChatroomClient(serverAddr)
	if err != nil {
		panic(err)
	}

	// Establish a connection to the server
	fmt.Printf("Connecting to %s...\n", serverAddr.String())
	if err := chatroomClient.connection.Connect(
		context.Background(),
	); err != nil {
		log.Fatalf("Couldn't connect to the server: %s", err)
	}
	fmt.Println("Connected successfully!")

	// Authenticate if credentials are already provided from the CLI
	if *username != "" && *password != "" {
		chatroomClient.Authenticate(*username, *password)
	}

	// Start the main loop
	chatroomClient.Start()
}
