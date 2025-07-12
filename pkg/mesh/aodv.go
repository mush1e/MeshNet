package mesh

import (
	"sync"
	"time"

	"github.com/mush1e/meshNet/pkg/protocol"
)

// PendingRequest tracks a route discovery in progress
type PendingRequest struct {
	Destination protocol.NodeID
	RequestID   uint32
	Retries     int
	StartTime   time.Time
	Timeout     *time.Timer
	Callbacks   []func(route *protocol.AODVRoute, err error) // Support multiple callbacks
}

// AODV engine handles route discovery and
// maintainance using the AODV protocol
type AODVEngine struct {
	localNodeID     protocol.NodeID
	seqNum          uint32
	requestID       uint32
	routingTable    map[protocol.NodeID]*protocol.AODVRoute // - maps destination to route info
	requestTable    map[string]*protocol.RequestEntry       // - tracks requests we've seen to prevent loops
	pendingRequests map[protocol.NodeID]*PendingRequest     // - routes we're currently discovering

	mutex sync.RWMutex

	// Callbacks for sending messages via transport
	sendMessage func(to protocol.NodeID, data []byte) error
	broadcast   func(data []byte) error

	// Cleanup ticker
	cleanupTicker *time.Ticker
	stopCleanup   chan bool
}

// NewAODVEngine creates a new AODV routing engine
func NewAODVEngine(nodeID protocol.NodeID, sendMsg func(protocol.NodeID, []byte) error, broadcast func([]byte) error) *AODVEngine {
	engine := &AODVEngine{
		localNodeID:     nodeID,
		seqNum:          1, // Start with sequence number 1
		requestID:       1,
		routingTable:    make(map[protocol.NodeID]*protocol.AODVRoute),
		requestTable:    make(map[string]*protocol.RequestEntry),
		pendingRequests: make(map[protocol.NodeID]*PendingRequest),
		sendMessage:     sendMsg,
		broadcast:       broadcast,
		stopCleanup:     make(chan bool),
	}

	return engine
}
