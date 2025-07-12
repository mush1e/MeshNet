// Package mesh provides the core mesh networking functionality.
// This layer handles message routing, peer management, and network topology
// while abstracting away transport-specific details.
package mesh

import (
	"context"
	"errors"

	"github.com/mush1e/meshNet/pkg/protocol"
	"github.com/mush1e/meshNet/pkg/transport"
)

// Common errors
var (
	ErrNodeNotFound    = errors.New("destination node not found")
	ErrMessageTooLarge = errors.New("message exceeds maximum size")
	ErrInvalidTTL      = errors.New("invalid TTL value")
	ErrMeshNotStarted  = errors.New("mesh not started")
	ErrAlreadyStarted  = errors.New("mesh already started")
)

// Mesh defines the interface for mesh network operations.
// This is what applications use to send messages and manage the network.
type Mesh interface {
	// Start initializes the mesh with the given transport
	Start(ctx context.Context, transport transport.Transport) error

	// Stop cleanly shuts down the mesh
	Stop() error

	// SendMessage sends a message to a specific peer
	// The mesh handles routing through intermediate nodes if needed
	SendMessage(to protocol.NodeID, data []byte, priority protocol.Priority) error

	// BroadcastMessage sends a message to all reachable peers
	// Priority determines flood behavior and retry count
	BroadcastMessage(data []byte, priority protocol.Priority) error

	// GetPeers returns a snapshot of all known peers
	GetPeers() map[protocol.NodeID]*protocol.Peer

	// GetNetworkTopology returns the current network map
	// Useful for visualization and debugging
	GetNetworkTopology() *protocol.NetworkMap

	// IncomingMessages returns a channel for receiving messages
	// Applications listen on this channel for incoming data
	IncomingMessages() <-chan *protocol.Message

	// ConnectionStatus returns a channel for peer status updates
	// Applications can react to nodes joining/leaving the network
	ConnectionStatus() <-chan *protocol.PeerStatus

	// GetLocalNodeID returns this node's identifier
	GetLocalNodeID() protocol.NodeID

	// IsConnected checks if we have a route to a specific peer
	IsConnected(nodeID protocol.NodeID) bool
}

// MeshConfig holds configuration for the mesh layer
type MeshConfig struct {
	// LocalNodeID is this node's unique identifier
	LocalNodeID protocol.NodeID

	// LocalNodeName is a human-readable name
	LocalNodeName string

	// MaxRetransmissions per priority level (overrides protocol defaults)
	MaxRetransmissions map[protocol.Priority]int

	// RouteTimeoutSeconds - how long to keep unused routes
	RouteTimeoutSeconds int

	// PeerTimeoutSeconds - how long before considering a peer dead
	PeerTimeoutSeconds int

	// EnableRouteDiscovery - whether to actively discover routes
	EnableRouteDiscovery bool
}

// DefaultConfig returns a production-ready mesh configuration
func DefaultConfig(nodeID protocol.NodeID, nodeName string) *MeshConfig {
	return &MeshConfig{
		LocalNodeID:   nodeID,
		LocalNodeName: nodeName,
		MaxRetransmissions: map[protocol.Priority]int{
			protocol.PriorityLow:       protocol.MaxRetriesLow,
			protocol.PriorityNormal:    protocol.MaxRetriesNormal,
			protocol.PriorityHigh:      protocol.MaxRetriesHigh,
			protocol.PriorityEmergency: protocol.MaxRetriesEmergency,
		},
		RouteTimeoutSeconds:  300, // 5 minutes
		PeerTimeoutSeconds:   60,  // 1 minute
		EnableRouteDiscovery: true,
	}
}

// MessageStats holds statistics about message processing
// Useful for monitoring and debugging in emergency scenarios
type MessageStats struct {
	MessagesSent      uint64
	MessagesReceived  uint64
	MessagesForwarded uint64
	MessagesDropped   uint64
	RoutesDiscovered  uint64
	RoutesExpired     uint64
}

// NetworkHealth represents the overall health of the mesh network
type NetworkHealth struct {
	ConnectedPeers    int
	KnownPeers        int
	ActiveRoutes      int
	MessageLatencyMs  float64 // Average message latency
	PacketLossPercent float64 // Percentage of lost messages
}

// RouteInfo provides detailed information about a specific route
type RouteInfo struct {
	Destination protocol.NodeID
	Path        []protocol.NodeID // Complete path through the mesh
	Hops        int
	Latency     int64   // Milliseconds
	Reliability float64 // Success rate (0.0 to 1.0)
	LastUsed    int64   // Unix timestamp
}

// Additional interface for monitoring and debugging
// Emergency systems need visibility into network state
type MeshMonitor interface {
	// GetStats returns message processing statistics
	GetStats() *MessageStats

	// GetNetworkHealth returns overall network health metrics
	GetNetworkHealth() *NetworkHealth

	// GetRouteInfo returns detailed info about routes to a peer
	GetRouteInfo(nodeID protocol.NodeID) (*RouteInfo, error)

	// GetAllRoutes returns info about all known routes
	GetAllRoutes() map[protocol.NodeID]*RouteInfo
}
