package mesh

import (
	"encoding/json"
	"fmt"
	"log"
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

// Stop shuts down the AODV engine
func (a *AODVEngine) Stop() {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Stop cleanup routine
	if a.cleanupTicker != nil {
		a.cleanupTicker.Stop()
		close(a.stopCleanup)
	}

	// Cancel all pending requests
	for dest, pending := range a.pendingRequests {
		if pending.Timeout != nil {
			pending.Timeout.Stop()
		}
		for _, callback := range pending.Callbacks {
			callback(nil, fmt.Errorf("AODV engine stopped"))
		}
		delete(a.pendingRequests, dest)
	}
}

// FindRoute discovers a route to the destination using AODV
// Returns immediately if route exists, otherwise initiates route discovery
func (a *AODVEngine) FindRoute(destination protocol.NodeID, callback func(*protocol.AODVRoute, error)) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Don't route to ourselves
	if destination == a.localNodeID {
		callback(nil, fmt.Errorf("cannot route to self"))
		return
	}

	// Check if we already have a valid route
	if route, exists := a.routingTable[destination]; exists && route.Active && time.Now().Before(route.Lifetime) {
		// Update last used time
		route.LastUsed = time.Now()
		callback(route, nil)
		return
	}

	// Check if we're already discovering this route
	if pending, exists := a.pendingRequests[destination]; exists {
		// Add callback to existing request
		pending.Callbacks = append(pending.Callbacks, callback)
		return
	}

	// Start new route discovery
	a.initiateRouteDiscovery(destination, callback)
}

// initiateRouteDiscovery starts AODV route discovery process
func (a *AODVEngine) initiateRouteDiscovery(destination protocol.NodeID, callback func(*protocol.AODVRoute, error)) {
	// Increment our request ID
	a.requestID++
	if a.requestID == 0 { // Handle overflow
		a.requestID = 1
	}

	// Create route request
	rreq := &protocol.RouteRequest{
		Type:        protocol.AODVRouteRequest,
		RequestID:   a.requestID,
		Source:      a.localNodeID,
		Destination: destination,
		SourceSeq:   a.seqNum,
		DestSeq:     0, // Unknown destination sequence
		HopCount:    0,
		Created:     time.Now(),
	}

	// Track this pending request
	timeout := time.NewTimer(protocol.RequestTimeout)
	pending := &PendingRequest{
		Destination: destination,
		RequestID:   a.requestID,
		Retries:     0,
		StartTime:   time.Now(),
		Timeout:     timeout,
		Callbacks:   []func(*protocol.AODVRoute, error){callback},
	}

	a.pendingRequests[destination] = pending

	// Set up timeout handler
	go func() {
		<-timeout.C
		a.handleRequestTimeout(destination)
	}()

	// Broadcast the route request
	if err := a.broadcastRouteRequest(rreq); err != nil {
		delete(a.pendingRequests, destination)
		callback(nil, fmt.Errorf("failed to broadcast route request: %v", err))
	}
}

// ProcessAODVMessage handles incoming AODV messages
func (a *AODVEngine) ProcessAODVMessage(msg []byte, from protocol.NodeID) error {
	var aodvMsg protocol.AODVMessage
	if err := json.Unmarshal(msg, &aodvMsg); err != nil {
		return fmt.Errorf("failed to parse AODV message: %v", err)
	}

	switch aodvMsg.Type {
	case protocol.AODVRouteRequest:
		return a.handleRouteRequest(aodvMsg.Data, from)
	case protocol.AODVRouteReply:
		return a.handleRouteReply(aodvMsg.Data, from)
	case protocol.AODVRouteError:
		return a.handleRouteError(aodvMsg.Data, from)
	default:
		return fmt.Errorf("unknown AODV message type: %d", aodvMsg.Type)
	}
}

// handleRouteRequest processes incoming RREQ messages
func (a *AODVEngine) handleRouteRequest(data []byte, from protocol.NodeID) error {
	var rreq protocol.RouteRequest
	if err := json.Unmarshal(data, &rreq); err != nil {
		return err
	}

	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Check if we've seen this request before (loop prevention)
	requestKey := fmt.Sprintf("%s-%d", rreq.Source, rreq.RequestID)
	if _, seen := a.requestTable[requestKey]; seen {
		return nil // Drop duplicate request
	}

	// Record that we've seen this request
	a.requestTable[requestKey] = &protocol.RequestEntry{
		RequestID: rreq.RequestID,
		Source:    rreq.Source,
		Created:   time.Now(),
	}

	// Update/create reverse route to source
	a.updateRoutingTable(rreq.Source, from, rreq.SourceSeq, rreq.HopCount+1)

	// Are we the destination?
	if rreq.Destination == a.localNodeID {
		// Send route reply back to source
		return a.sendRouteReply(rreq, from)
	}

	// Check if we have a fresh route to the destination
	if route, exists := a.routingTable[rreq.Destination]; exists && route.Active &&
		time.Now().Before(route.Lifetime) && route.SeqNum >= rreq.DestSeq {
		// We can answer with a route reply
		return a.sendIntermediateRouteReply(rreq, route, from)
	}

	// Forward the request (increment hop count)
	rreq.HopCount++
	return a.broadcastRouteRequest(&rreq)
}

// handleRouteReply processes incoming RREP messages
func (a *AODVEngine) handleRouteReply(data []byte, from protocol.NodeID) error {
	var rrep protocol.RouteReply
	if err := json.Unmarshal(data, &rrep); err != nil {
		return err
	}

	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Update route to destination
	lifetime := time.Now().Add(rrep.Lifetime)
	a.updateRoutingTableWithLifetime(rrep.Destination, from, rrep.DestSeq, rrep.HopCount+1, lifetime)

	// Are we the original source of this request?
	if rrep.Source == a.localNodeID {
		// Route discovery complete!
		if pending, exists := a.pendingRequests[rrep.Destination]; exists {
			route := a.routingTable[rrep.Destination]

			// Cancel timeout
			if pending.Timeout != nil {
				pending.Timeout.Stop()
			}

			// Notify all waiting callbacks
			for _, callback := range pending.Callbacks {
				callback(route, nil)
			}

			delete(a.pendingRequests, rrep.Destination)
		}
		return nil
	}

	// Forward the route reply toward the source
	if sourceRoute, exists := a.routingTable[rrep.Source]; exists && sourceRoute.Active {
		return a.forwardRouteReply(&rrep, sourceRoute.NextHop)
	}

	log.Printf("Cannot forward RREP to %s: no route to source", rrep.Source)
	return nil
}

// handleRouteError processes incoming RERR messages
func (a *AODVEngine) handleRouteError(data []byte, from protocol.NodeID) error {
	var rerr protocol.RouteError
	if err := json.Unmarshal(data, &rerr); err != nil {
		return err
	}

	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Mark unreachable destinations as inactive
	affectedSources := make(map[protocol.NodeID]bool)

	for i, unreachableNode := range rerr.UnreachableNodes {
		if route, exists := a.routingTable[unreachableNode]; exists && route.NextHop == from {
			// This route goes through the failed link
			route.Active = false

			// Find sources that use this route
			for dest, r := range a.routingTable {
				if r.NextHop == unreachableNode && r.Active {
					r.Active = false
					affectedSources[dest] = true
				}
			}

			// Update sequence number if provided
			if i < len(rerr.DestSeq) {
				route.SeqNum = rerr.DestSeq[i]
			}
		}
	}

	// Propagate route error to affected sources
	if len(affectedSources) > 0 {
		return a.propagateRouteError(affectedSources)
	}

	return nil
}

// sendRouteReply sends a RREP message back to the source (when we are the destination)
func (a *AODVEngine) sendRouteReply(rreq protocol.RouteRequest, from protocol.NodeID) error {
	a.seqNum++ // Increment our sequence number

	rrep := &protocol.RouteReply{
		Type:        protocol.AODVRouteReply,
		Source:      rreq.Source,
		Destination: a.localNodeID,
		DestSeq:     a.seqNum,
		HopCount:    0,
		Lifetime:    protocol.DefaultRouteLifetime,
		Created:     time.Now(),
	}

	return a.sendAODVMessage(from, protocol.AODVRouteReply, rrep)
}

// sendIntermediateRouteReply sends RREP when we have a route to destination
func (a *AODVEngine) sendIntermediateRouteReply(rreq protocol.RouteRequest, route *protocol.AODVRoute, from protocol.NodeID) error {
	rrep := &protocol.RouteReply{
		Type:        protocol.AODVRouteReply,
		Source:      rreq.Source,
		Destination: rreq.Destination,
		DestSeq:     route.SeqNum,
		HopCount:    route.HopCount,
		Lifetime:    time.Until(route.Lifetime),
		Created:     time.Now(),
	}

	return a.sendAODVMessage(from, protocol.AODVRouteReply, rrep)
}

// forwardRouteReply forwards RREP toward the original source
func (a *AODVEngine) forwardRouteReply(rrep *protocol.RouteReply, nextHop protocol.NodeID) error {
	rrep.HopCount++
	return a.sendAODVMessage(nextHop, protocol.AODVRouteReply, rrep)
}

// handleRequestTimeout handles route request timeouts with retries
func (a *AODVEngine) handleRequestTimeout(destination protocol.NodeID) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	pending, exists := a.pendingRequests[destination]
	if !exists {
		return // Request already completed
	}

	pending.Retries++
	if pending.Retries >= protocol.MaxRequestRetries {
		// Give up - notify callbacks of failure
		for _, callback := range pending.Callbacks {
			callback(nil, fmt.Errorf("route discovery failed: destination unreachable"))
		}
		delete(a.pendingRequests, destination)
		return
	}

	// Retry with new request ID
	a.requestID++
	if a.requestID == 0 {
		a.requestID = 1
	}

	rreq := &protocol.RouteRequest{
		Type:        protocol.AODVRouteRequest,
		RequestID:   a.requestID,
		Source:      a.localNodeID,
		Destination: destination,
		SourceSeq:   a.seqNum,
		DestSeq:     0,
		HopCount:    0,
		Created:     time.Now(),
	}

	// Set new timeout
	pending.RequestID = a.requestID
	pending.Timeout = time.NewTimer(protocol.RequestTimeout)

	go func() {
		<-pending.Timeout.C
		a.handleRequestTimeout(destination)
	}()

	// Retry broadcast
	a.broadcastRouteRequest(rreq)
}

// Helper methods for message serialization and sending
func (a *AODVEngine) broadcastRouteRequest(rreq *protocol.RouteRequest) error {
	return a.sendAODVMessage("", protocol.AODVRouteRequest, rreq)
}

func (a *AODVEngine) sendAODVMessage(to protocol.NodeID, msgType protocol.AODVMessageType, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	aodvMsg := protocol.AODVMessage{
		Type: msgType,
		Data: data,
	}

	msgBytes, err := json.Marshal(aodvMsg)
	if err != nil {
		return err
	}

	if to == "" {
		return a.broadcast(msgBytes)
	}
	return a.sendMessage(to, msgBytes)
}

// updateRoutingTable adds or updates a route entry
func (a *AODVEngine) updateRoutingTable(dest, nextHop protocol.NodeID, seqNum uint32, hopCount int) {
	lifetime := time.Now().Add(protocol.DefaultRouteLifetime)
	a.updateRoutingTableWithLifetime(dest, nextHop, seqNum, hopCount, lifetime)
}

func (a *AODVEngine) updateRoutingTableWithLifetime(dest, nextHop protocol.NodeID, seqNum uint32, hopCount int, lifetime time.Time) {
	now := time.Now()

	route := &protocol.AODVRoute{
		BaseRoute: protocol.BaseRoute{
			Destination: dest,
			NextHop:     nextHop,
			HopCount:    hopCount,
			Cost:        hopCount, // Simple cost metric
			Created:     now,
			LastUsed:    now,
		},
		SeqNum:   seqNum,
		Lifetime: lifetime,
		Active:   true,
	}

	a.routingTable[dest] = route
}

// propagateRouteError creates and sends RERR for affected destinations
func (a *AODVEngine) propagateRouteError(affectedDests map[protocol.NodeID]bool) error {
	unreachableNodes := make([]protocol.NodeID, 0, len(affectedDests))
	destSeqs := make([]uint32, 0, len(affectedDests))

	for dest := range affectedDests {
		unreachableNodes = append(unreachableNodes, dest)
		if route, exists := a.routingTable[dest]; exists {
			destSeqs = append(destSeqs, route.SeqNum)
		} else {
			destSeqs = append(destSeqs, 0)
		}
	}

	rerr := &protocol.RouteError{
		Type:             protocol.AODVRouteError,
		UnreachableNodes: unreachableNodes,
		DestSeq:          destSeqs,
		Created:          time.Now(),
	}

	return a.sendAODVMessage("", protocol.AODVRouteError, rerr)
}

// cleanupRoutine periodically removes expired routes and old requests
func (a *AODVEngine) cleanupRoutine() {
	for {
		select {
		case <-a.stopCleanup:
			return
		case <-a.cleanupTicker.C:
			a.cleanup()
		}
	}
}

func (a *AODVEngine) cleanup() {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	now := time.Now()

	// Clean expired routes
	for dest, route := range a.routingTable {
		if now.After(route.Lifetime) {
			log.Printf("Route to %s expired", dest)
			delete(a.routingTable, dest)
		}
	}

	// Clean old request entries
	for key, entry := range a.requestTable {
		if now.Sub(entry.Created) > protocol.RequestTableCleanup {
			delete(a.requestTable, key)
		}
	}
}

// GetRoutingTable returns a copy of the current routing table (for debugging)
func (a *AODVEngine) GetRoutingTable() map[protocol.NodeID]*protocol.AODVRoute {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	result := make(map[protocol.NodeID]*protocol.AODVRoute)
	for k, v := range a.routingTable {
		routeCopy := *v
		result[k] = &routeCopy
	}
	return result
}
