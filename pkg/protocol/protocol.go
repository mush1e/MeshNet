package protocol

import "time"

// Priority represents message broadcast
// range and frequency
type Priority uint8

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityEmergency // Flood networks
)

// String returns the string representation of Priority
func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "LOW"
	case PriorityNormal:
		return "NORMAL"
	case PriorityHigh:
		return "HIGH"
	case PriorityEmergency:
		return "EMERGENCY"
	default:
		return "UNKNOWN"
	}
}

// NodeID represents a unique identifier
// for each node in our mesh network
type NodeID string

// Peer represents information pertaining
// to a peer node in our network
type Peer struct {
	ID       NodeID    `json:"id"`
	Name     string    `json:"name"`
	Addr     string    `json:"addr"`
	Port     int       `json:"port"`
	LastSeen time.Time `json:"last_seen"`
}

// Message represents an application-level
// message to be sent through the mesh
type Message struct {
	ID       string    `json:"id"`       // Unique message identifier
	From     NodeID    `json:"from"`     // Source node
	To       NodeID    `json:"to"`       // Destination node (empty for broadcast)
	Data     []byte    `json:"data"`     // Application payload
	Priority Priority  `json:"priority"` // Message priority
	TTL      int       `json:"ttl"`      // Time to live (hops remaining) -> TO PREVENT INF LOOPS
	Created  time.Time `json:"created"`  // When message was created
}

// Route represents a path through our mesh network
type Route struct {
	Destination NodeID    `json:"destination"`
	NextHop     NodeID    `json:"next_hop"`
	Hops        int       `json:"hops"`
	Cost        int       `json:"cost"` // Routing cost metric
	LastUsed    time.Time `json:"last_used"`
}

// PeerStatus represents the connection status of a peer
type PeerStatus struct {
	Peer      *Peer     `json:"peer"`
	Connected bool      `json:"connected"`
	LastSeen  time.Time `json:"last_seen"`
	Error     string    `json:"error,omitempty"` // Error message if disconnected
}

// NetworkMap represents the topology of the mesh network
type NetworkMap struct {
	LocalNode NodeID            `json:"local_node"`
	Peers     map[NodeID]*Peer  `json:"peers"`
	Routes    map[NodeID]*Route `json:"routes"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Constants for mesh operation
const (
	// MaxMessageSize limits message payload size to prevent memory exhaustion
	MaxMessageSize = 1024 * 1024 // 1MB

	// MaxTTL limits how far messages can propagate
	MaxTTL = 30

	// DefaultTTL for normal messages
	DefaultTTL = 10

	// EmergencyTTL for emergency messages
	EmergencyTTL = 20

	// MaxRetries per priority level
	MaxRetriesLow       = 1
	MaxRetriesNormal    = 2
	MaxRetriesHigh      = 3
	MaxRetriesEmergency = 5
)
