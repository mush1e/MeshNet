package transport

import (
	"context"

	"github.com/mush1e/meshNet/pkg/protocol"
)

// Transport defines the interface that all transport implementations must satisfy.
// This abstraction allows the mesh layer to work with different underlying
// network technologies (TCP, WiFi Direct, Bluetooth, radio, etc.).
type Transport interface {
	// Start initializes the transport and begins listening for peers and messages.
	// It should start both passive discovery (listening) and prepare for active discovery.
	Start(ctx context.Context) error

	// Stop cleanly shuts down the transport, closing all connections.
	Stop() error

	// SendTo sends a message to a specific peer identified by NodeID.
	// Returns error if peer is not reachable or message fails to send.
	SendTo(peerID protocol.NodeID, data []byte) error

	// Broadcast sends a message to all currently connected peers.
	// This is used for emergency broadcasts and network announcements.
	Broadcast(data []byte) error

	// DiscoverPeers actively searches for new peers.
	// This is optional - transports may choose to only do passive discovery.
	DiscoverPeers() error

	// PeerDiscovered returns a channel that receives newly discovered peers.
	// The mesh layer listens on this channel to learn about the network topology.
	PeerDiscovered() <-chan *protocol.Peer

	// MessageReceived returns a channel that receives incoming messages.
	// The mesh layer processes these messages for routing or delivery.
	MessageReceived() <-chan *TransportMessage

	// ConnectionStatusChanged returns a channel that receives peer status updates.
	// This allows the mesh to react to connection failures and recoveries.
	ConnectionStatusChanged() <-chan *protocol.PeerStatus

	// GetConnectedPeers returns a snapshot of currently connected peers.
	// Useful for debugging and network visualization.
	GetConnectedPeers() map[protocol.NodeID]*protocol.Peer
}

// TransportMessage wraps an incoming message with metadata about how it was received.
// This gives the mesh layer context about the message's origin and transport details.
type TransportMessage struct {
	// Data is the raw message payload received from the peer
	Data []byte

	// From identifies which peer sent this message
	From protocol.NodeID

	// ReceivedAt indicates when this message was received (for debugging/metrics)
	ReceivedAt int64 // Unix timestamp

	// Transport-specific metadata (optional, can be nil)
	Metadata map[string]interface{}
}

// Config represents transport-specific configuration.
// Different transport implementations will embed this or define their own config types.
type Config struct {
	// LocalNodeID is this node's unique identifier
	LocalNodeID protocol.NodeID

	// LocalNodeName is a human-readable name for this node
	LocalNodeName string

	// Port is the port to listen on (for IP-based transports)
	Port int

	// DiscoveryInterval controls how often to actively discover peers (seconds)
	DiscoveryInterval int

	// HeartbeatInterval controls how often to send keepalive messages (seconds)
	HeartbeatInterval int

	// ConnectionTimeout controls how long to wait for connections (seconds)
	ConnectionTimeout int
}

// Default configuration values for production use
const (
	DefaultPort              = 8001
	DefaultDiscoveryInterval = 30 // 30 seconds
	DefaultHeartbeatInterval = 10 // 10 seconds
	DefaultConnectionTimeout = 15 // 15 seconds
)
