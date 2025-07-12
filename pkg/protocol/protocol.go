package protocol

import (
	"time"
)

// Priority defines message priority levels for messages
type Priority uint8

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityEmergency // Floods aggressively, highest retry count
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

// NodeID represents a unique identifier for a node in the mesh
type NodeID string

// Peer represents information about a peer node
type Peer struct {
	ID       NodeID    `json:"id"`
	Name     string    `json:"name"`
	Addr     string    `json:"addr"`
	Port     int       `json:"port"`
	LastSeen time.Time `json:"last_seen"`
}

// Message represents an application-level message to be sent through the mesh
type Message struct {
	ID       string    `json:"id"`       // Unique message identifier
	From     NodeID    `json:"from"`     // Source node
	To       NodeID    `json:"to"`       // Destination node (empty for broadcast)
	Data     []byte    `json:"data"`     // Application payload
	Priority Priority  `json:"priority"` // Message priority
	TTL      int       `json:"ttl"`      // Time to live (hops remaining)
	Created  time.Time `json:"created"`  // When message was created
}

// BaseRoute contains common routing information used by all routing algorithms
type BaseRoute struct {
	Destination NodeID    `json:"destination"` // Where this route leads
	NextHop     NodeID    `json:"next_hop"`    // Next node in the path
	HopCount    int       `json:"hop_count"`   // Number of hops to destination
	Cost        int       `json:"cost"`        // Routing cost metric
	Created     time.Time `json:"created"`     // When route was established
	LastUsed    time.Time `json:"last_used"`   // When route was last used
}

// AODVRoute extends BaseRoute with AODV-specific information
type AODVRoute struct {
	BaseRoute
	SeqNum   uint32    `json:"seq_num"`  // Destination's sequence number
	Lifetime time.Time `json:"lifetime"` // When this route expires (absolute time)
	Active   bool      `json:"active"`   // Whether route is currently usable
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
	LocalNode NodeID                `json:"local_node"`
	Peers     map[NodeID]*Peer      `json:"peers"`
	Routes    map[NodeID]*AODVRoute `json:"routes"` // Using AODVRoute for now
	UpdatedAt time.Time             `json:"updated_at"`
}

// AODV Protocol Messages
// These are internal mesh protocol messages, not application messages

// AODVMessageType identifies the type of AODV message
type AODVMessageType uint8

const (
	AODVRouteRequest AODVMessageType = iota
	AODVRouteReply
	AODVRouteError
)

// RouteRequest (RREQ) - "Anyone know how to reach destination?"
type RouteRequest struct {
	Type        AODVMessageType `json:"type"`        // Always AODVRouteRequest
	RequestID   uint32          `json:"request_id"`  // Unique ID to prevent loops
	Source      NodeID          `json:"source"`      // Who's asking
	Destination NodeID          `json:"destination"` // Who they want to reach
	SourceSeq   uint32          `json:"source_seq"`  // Source's sequence number
	DestSeq     uint32          `json:"dest_seq"`    // Last known dest sequence
	HopCount    int             `json:"hop_count"`   // How many hops so far
	Created     time.Time       `json:"created"`     // When created (consistent time type)
}

// RouteReply (RREP) - "I'm the destination, here's the path back!"
type RouteReply struct {
	Type        AODVMessageType `json:"type"`        // Always AODVRouteReply
	Source      NodeID          `json:"source"`      // Original requester
	Destination NodeID          `json:"destination"` // The found destination
	DestSeq     uint32          `json:"dest_seq"`    // Destination's current sequence
	HopCount    int             `json:"hop_count"`   // Hops from dest to source
	Lifetime    time.Duration   `json:"lifetime"`    // How long this route is valid
	Created     time.Time       `json:"created"`     // When created
}

// RouteError (RERR) - "The path is broken!"
type RouteError struct {
	Type             AODVMessageType `json:"type"`              // Always AODVRouteError
	UnreachableNodes []NodeID        `json:"unreachable_nodes"` // Which nodes can't be reached
	DestSeq          []uint32        `json:"dest_seq"`          // Their sequence numbers
	Created          time.Time       `json:"created"`           // When error detected
}

// AODVMessage wraps all AODV messages for transport
type AODVMessage struct {
	Type AODVMessageType `json:"type"`
	Data []byte          `json:"data"` // JSON-encoded RouteRequest/RouteReply/RouteError
}

// RequestEntry tracks outgoing route requests to prevent loops
type RequestEntry struct {
	RequestID uint32    `json:"request_id"` // Unique request identifier
	Source    NodeID    `json:"source"`     // Who initiated the request
	Created   time.Time `json:"created"`    // When we saw this request (consistent naming)
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

// Constants for AODV operation
const (
	// MaxRequestID is the maximum request ID before wrapping
	MaxRequestID = ^uint32(0)

	// DefaultRouteLifetime for routes
	DefaultRouteLifetime = 5 * time.Minute

	// RequestTimeout - how long to wait for route replies
	RequestTimeout = 3 * time.Second

	// MaxRequestRetries before giving up
	MaxRequestRetries = 3

	// RequestTableCleanup - how often to clean old requests
	RequestTableCleanup = 1 * time.Minute

	// RouteTableCleanup - how often to clean expired routes
	RouteTableCleanup = 30 * time.Second
)
