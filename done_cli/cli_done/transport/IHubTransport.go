package transport

import (
	"crypto/x509"
	"errors"

	"github.com/hiveot/hub/done_tool/keys"
)

// ISubscription interface to underlying subscription mechanism
type ISubscription interface {
	Unsubscribe() error
}

type ConnectionStatus string

const (
	// Connecting attempting a (re)connection
	Connecting ConnectionStatus = "connecting"
	// Connected and authenticated successful
	Connected ConnectionStatus = "connected"
	// ConnectFailed after failure to connect
	// Only used if retry has given up
	ConnectFailed ConnectionStatus = "connectFailed"
	// Disconnected by client or not yet connected
	Disconnected ConnectionStatus = "disconnected"
	// Expired authentication token
	Expired ConnectionStatus = "expired"
	// Unauthorized login name or password
	Unauthorized ConnectionStatus = "unauthorized"
)

// MessageTypeINBOX special inbox prefix for RPCs
// reserved event and action names
const (
	MessageTypeAction = "action"
	MessageTypeConfig = "config"
	MessageTypeEvent  = "event"
	MessageTypeRPC    = "rpc"
	MessageTypeINBOX  = "_INBOX"
	EventNameTD       = "$td"
	EventNameProps    = "$properties"
)

var ErrorUnauthorized = errors.New(string(Unauthorized))

type HubTransportStatus struct {
	HubURL           string
	CaCert           *x509.Certificate
	ClientID         string
	ConnectionStatus ConnectionStatus
	LastError        error
}

// IHubTransport defines the interface of the transport that connects to the messaging server.
type IHubTransport interface {

	// AddressTokens returns the address separator and wildcard tokens used by the transport.
	//  sep is the address separator. eg "." for nats
	//  wc is the address wildcard - "*"
	//  rem is the address remainder. "" for nats
	AddressTokens() (sep, wc, rem string)

	// ConnectWithPassword connects to the messaging server using password authentication.
	//  loginID is the client's ID
	//  password is created when registering the user with the auth service.
	ConnectWithPassword(password string) error

	// ConnectWithToken connects to the messaging server using an authentication token
	// and pub/private keys provided when creating an instance of the hub client.
	//  kp is the client's key pair
	//  token is created by the auth service.
	ConnectWithToken(kp keys.IHiveKey, token string) error

	// CreateKeyPair returns a new set of serialized public/private key pair.
	//  serializedKP contains the serialized public/private key pair
	//  pubKey contains the serialized public key to be shared
	CreateKeyPair() (kp keys.IHiveKey)

	// Disconnect from the messaging server.
	// This removes all subscriptions.
	Disconnect()

	// GetStatus returns the current transport connection status
	GetStatus() HubTransportStatus

	// PubEvent publishes an event style message without waiting for a response.
	//	address to publish on
	//	payload with serialized message to publish
	PubEvent(address string, payload []byte) error

	// PubRequest publishes a request and waits for a response.
	//  address to publish on
	//  payload with serialized message to publish
	//  returns a reply with serialized response message
	PubRequest(address string, payload []byte) (reply []byte, err error)

	// SetConnectHandler sets the notification handler of connection status changes
	SetConnectHandler(cb func(status HubTransportStatus))

	// SetEventHandler set the single handler that receives all subscribed events.
	// Messages are considered events when they do not have a reply-to address.
	// This does not provide routing as in most cases it is unnecessary overhead
	// Use 'Subscribe' to set the addresses that this receives events on.
	SetEventHandler(cb func(addr string, payload []byte))

	// SetRequestHandler sets the handler that receives all subscribed requests.
	// Messages are considered requests when they have a reply-to address.
	// This does not provide routing as in most cases it is unnecessary overhead
	// Use 'Subscribe' to set the addresses that this receives requests on.
	SetRequestHandler(cb func(addr string, payload []byte) (reply []byte, err error, donotreply bool))

	// Subscribe adds a subscription for an event or request address.
	// Incoming messages are passed to the event handler or the request handler, depending on whether they
	// have a reply-to address. The event/request handler will handle the routing as this is application specific.
	// Subscriptions remain in effect when the connection with the messaging server is interrupted.
	//
	// The address MUST be constructed using the tokens provided by AddressTokens()
	Subscribe(address string) error

	// Unsubscribe removes a previous address subscription.
	// No more events or requests will be received after Unsubscribe.
	Unsubscribe(address string)

	//// SubEvent subscribes to an event style message.
	////	address to subscribe to, this can contain wildcards
	////	cb callback to invoke when a message is received
	//SubEvent(address string, cb func(addr string, data []byte)) (ISubscription, error)
	//
	//// SubRequest subscribes to RPC requests and sends the reply to the sender.
	//// Intended for services.
	////  address is the address to subscribe to (using AddressTokens to construct)
	////  cb is the callback to invoke when a message is received
	////
	//// Returns a subscription object that needs to be unsubscribed when done
	//SubRequest(address string, handler func(addr string, payload []byte) (
	//	reply []byte, err error)) (ISubscription, error)
}
