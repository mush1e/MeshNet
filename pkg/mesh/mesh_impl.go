// Package mesh provides the main mesh implementation that coordinates
// transport, routing, and application layers for emergency communications
package mesh

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/mush1e/meshNet/pkg/protocol"
	"github.com/mush1e/meshNet/pkg/transport"
)

// meshImpl is the main implementation of the Mesh interface
// It coordinates between transport, AODV routing, and applications
type meshImpl struct {
	config *MeshConfig

	// Core components
	transport transport.Transport
	aodv      *AODVEngine

	// Internal state
	peers   map[protocol.NodeID]*protocol.Peer
	running bool
	mutex   sync.RWMutex

	// Channels for applications
	incomingMessages chan *protocol.Message
	connectionStatus chan *protocol.PeerStatus

	// Internal coordination
	ctx        context.Context
	cancel     context.CancelFunc
	stopSignal chan bool

	// Statistics for monitoring
	stats      MessageStats
	statsMutex sync.RWMutex
}

// NewMesh creates a new mesh instance with the given configuration
func NewMesh(config *MeshConfig) Mesh {
	return &meshImpl{
		config:           config,
		peers:            make(map[protocol.NodeID]*protocol.Peer),
		incomingMessages: make(chan *protocol.Message, 100), // Buffered for emergency bursts
		connectionStatus: make(chan *protocol.PeerStatus, 50),
		stopSignal:       make(chan bool),
	}
}

// Start initializes the mesh with the given transport
func (m *meshImpl) Start(ctx context.Context, t transport.Transport) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.running {
		return ErrAlreadyStarted
	}

	m.transport = t
	m.ctx, m.cancel = context.WithCancel(ctx)

	// Initialize AODV engine with transport callbacks
	m.aodv = NewAODVEngine(
		m.config.LocalNodeID,
		m.sendToTransport,      // Send to specific peer
		m.broadcastToTransport, // Broadcast to all peers
	)

	// Start transport
	if err := m.transport.Start(m.ctx); err != nil {
		return fmt.Errorf("failed to start transport: %v", err)
	}

	// Start internal goroutines
	go m.handleTransportEvents()
	go m.handleTransportMessages()
	go m.handleTransportPeerStatus()
	go m.periodicMaintenance()

	m.running = true
	log.Printf("Mesh started - Node ID: %s", m.config.LocalNodeID)

	return nil
}

// Stop cleanly shuts down the mesh
func (m *meshImpl) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.running {
		return ErrMeshNotStarted
	}

	log.Printf("Stopping mesh - Node ID: %s", m.config.LocalNodeID)

	// Signal stop to all goroutines
	m.cancel()
	close(m.stopSignal)

	// Stop AODV engine
	if m.aodv != nil {
		m.aodv.Stop()
	}

	// Stop transport
	if m.transport != nil {
		m.transport.Stop()
	}

	// Close channels
	close(m.incomingMessages)
	close(m.connectionStatus)

	m.running = false
	return nil
}

// SendMessage sends a message to a specific peer with routing
func (m *meshImpl) SendMessage(to protocol.NodeID, data []byte, priority protocol.Priority) error {
	if !m.running {
		return ErrMeshNotStarted
	}

	if len(data) > protocol.MaxMessageSize {
		return ErrMessageTooLarge
	}

	// Create application message
	msg := &protocol.Message{
		ID:       generateMessageID(),
		From:     m.config.LocalNodeID,
		To:       to,
		Data:     data,
		Priority: priority,
		TTL:      getTTLForPriority(priority),
		Created:  time.Now(),
	}

	// Try to send directly first (if we have direct connection)
	if m.isDirectlyConnected(to) {
		return m.sendDirectMessage(msg)
	}

	// Need routing - use AODV to find route
	return m.sendWithRouting(msg)
}

// BroadcastMessage sends a message to all reachable peers
func (m *meshImpl) BroadcastMessage(data []byte, priority protocol.Priority) error {
	if !m.running {
		return ErrMeshNotStarted
	}

	if len(data) > protocol.MaxMessageSize {
		return ErrMessageTooLarge
	}

	// Create broadcast message (empty To field)
	msg := &protocol.Message{
		ID:       generateMessageID(),
		From:     m.config.LocalNodeID,
		To:       "", // Empty = broadcast
		Data:     data,
		Priority: priority,
		TTL:      getTTLForPriority(priority),
		Created:  time.Now(),
	}

	// Serialize and broadcast via transport
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to serialize broadcast message: %v", err)
	}

	// Update stats
	m.updateStats(func(s *MessageStats) { s.MessagesSent++ })

	return m.transport.Broadcast(msgBytes)
}

// sendWithRouting uses AODV to find a route and send the message
func (m *meshImpl) sendWithRouting(msg *protocol.Message) error {
	// Use AODV to find route
	routeFound := make(chan error, 1)

	m.aodv.FindRoute(msg.To, func(route *protocol.AODVRoute, err error) {
		if err != nil {
			routeFound <- err
			return
		}

		// Route found - send via next hop
		msgBytes, serErr := json.Marshal(msg)
		if serErr != nil {
			routeFound <- fmt.Errorf("failed to serialize message: %v", serErr)
			return
		}

		// Send to next hop
		if sendErr := m.transport.SendTo(route.NextHop, msgBytes); sendErr != nil {
			routeFound <- fmt.Errorf("failed to send via route: %v", sendErr)
			return
		}

		// Success!
		m.updateStats(func(s *MessageStats) { s.MessagesSent++ })
		routeFound <- nil
	})

	// Wait for route discovery with timeout
	select {
	case err := <-routeFound:
		return err
	case <-time.After(10 * time.Second): // Emergency timeout
		return fmt.Errorf("route discovery timeout for %s", msg.To)
	case <-m.ctx.Done():
		return fmt.Errorf("mesh shutting down")
	}
}

// sendDirectMessage sends a message directly via transport (no routing needed)
func (m *meshImpl) sendDirectMessage(msg *protocol.Message) error {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to serialize message: %v", err)
	}

	m.updateStats(func(s *MessageStats) { s.MessagesSent++ })
	return m.transport.SendTo(msg.To, msgBytes)
}

// handleTransportMessages processes incoming messages from transport
func (m *meshImpl) handleTransportMessages() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case transportMsg := <-m.transport.MessageReceived():
			m.processIncomingMessage(transportMsg)
		}
	}
}

// processIncomingMessage determines if message is AODV protocol or application data
func (m *meshImpl) processIncomingMessage(transportMsg *transport.TransportMessage) {
	// Try to parse as AODV message first
	var aodvMsg protocol.AODVMessage
	if err := json.Unmarshal(transportMsg.Data, &aodvMsg); err == nil {
		// It's an AODV protocol message
		if err := m.aodv.ProcessAODVMessage(transportMsg.Data, transportMsg.From); err != nil {
			log.Printf("Error processing AODV message from %s: %v", transportMsg.From, err)
		}
		return
	}

	// Try to parse as application message
	var appMsg protocol.Message
	if err := json.Unmarshal(transportMsg.Data, &appMsg); err != nil {
		log.Printf("Failed to parse message from %s: %v", transportMsg.From, err)
		return
	}

	// Handle application message
	m.handleApplicationMessage(&appMsg, transportMsg.From)
}

// handleApplicationMessage processes application-level messages
func (m *meshImpl) handleApplicationMessage(msg *protocol.Message, receivedFrom protocol.NodeID) {
	// Check TTL
	if msg.TTL <= 0 {
		log.Printf("Dropping message %s: TTL exceeded", msg.ID)
		m.updateStats(func(s *MessageStats) { s.MessagesDropped++ })
		return
	}

	// Is this message for us?
	if msg.To == m.config.LocalNodeID || msg.To == "" { // Direct message or broadcast
		// Deliver to application
		select {
		case m.incomingMessages <- msg:
			m.updateStats(func(s *MessageStats) { s.MessagesReceived++ })
		default:
			log.Printf("Incoming message buffer full, dropping message %s", msg.ID)
			m.updateStats(func(s *MessageStats) { s.MessagesDropped++ })
		}

		// For broadcasts, also forward to other peers
		if msg.To == "" {
			m.forwardBroadcast(msg, receivedFrom)
		}
		return
	}

	// Message is for someone else - try to forward it
	m.forwardMessage(msg, receivedFrom)
}

// forwardMessage forwards a message toward its destination
func (m *meshImpl) forwardMessage(msg *protocol.Message, receivedFrom protocol.NodeID) {
	// Decrement TTL
	msg.TTL--
	if msg.TTL <= 0 {
		log.Printf("Not forwarding message %s: TTL would exceed", msg.ID)
		return
	}

	// Try direct connection first
	if m.isDirectlyConnected(msg.To) && msg.To != receivedFrom {
		if err := m.sendDirectMessage(msg); err == nil {
			m.updateStats(func(s *MessageStats) { s.MessagesForwarded++ })
			return
		}
	}

	// Use AODV routing
	m.aodv.FindRoute(msg.To, func(route *protocol.AODVRoute, err error) {
		if err != nil {
			log.Printf("Cannot forward message %s to %s: no route found", msg.ID, msg.To)
			m.updateStats(func(s *MessageStats) { s.MessagesDropped++ })
			return
		}

		// Don't send back to where it came from
		if route.NextHop == receivedFrom {
			log.Printf("Not forwarding message %s back to sender %s", msg.ID, receivedFrom)
			return
		}

		// Forward via next hop
		msgBytes, _ := json.Marshal(msg)
		if err := m.transport.SendTo(route.NextHop, msgBytes); err != nil {
			log.Printf("Failed to forward message %s: %v", msg.ID, err)
			m.updateStats(func(s *MessageStats) { s.MessagesDropped++ })
		} else {
			m.updateStats(func(s *MessageStats) { s.MessagesForwarded++ })
		}
	})
}

// forwardBroadcast forwards a broadcast message to other peers
func (m *meshImpl) forwardBroadcast(msg *protocol.Message, receivedFrom protocol.NodeID) {
	// Decrement TTL
	msg.TTL--
	if msg.TTL <= 0 {
		return
	}

	// Forward to all peers except sender
	m.mutex.RLock()
	peers := make([]protocol.NodeID, 0, len(m.peers))
	for id := range m.peers {
		if id != receivedFrom {
			peers = append(peers, id)
		}
	}
	m.mutex.RUnlock()

	// Send to each peer
	msgBytes, _ := json.Marshal(msg)
	for _, peerID := range peers {
		if err := m.transport.SendTo(peerID, msgBytes); err != nil {
			log.Printf("Failed to forward broadcast to %s: %v", peerID, err)
		}
	}
}

// handleTransportPeerStatus processes peer status updates from transport
func (m *meshImpl) handleTransportPeerStatus() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case status := <-m.transport.ConnectionStatusChanged():
			m.processPeerStatusUpdate(status)
		}
	}
}

// processPeerStatusUpdate handles peer connection/disconnection events
func (m *meshImpl) processPeerStatusUpdate(status *protocol.PeerStatus) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if status.Connected {
		// Peer connected
		m.peers[status.Peer.ID] = status.Peer
		log.Printf("Peer connected: %s (%s)", status.Peer.Name, status.Peer.ID)
	} else {
		// Peer disconnected - clean up
		delete(m.peers, status.Peer.ID)
		log.Printf("Peer disconnected: %s (%s)", status.Peer.Name, status.Peer.ID)

		// TODO: Trigger AODV route error for routes through this peer
	}

	// Forward to application
	select {
	case m.connectionStatus <- status:
	default:
		log.Printf("Connection status buffer full, dropping status update")
	}
}

// handleTransportEvents processes peer discovery events
func (m *meshImpl) handleTransportEvents() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case peer := <-m.transport.PeerDiscovered():
			m.processPeerDiscovered(peer)
		}
	}
}

// processPeerDiscovered handles newly discovered peers
func (m *meshImpl) processPeerDiscovered(peer *protocol.Peer) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if peer.ID == m.config.LocalNodeID {
		return // Don't add ourselves
	}

	// Add to peer list
	m.peers[peer.ID] = peer
	log.Printf("Discovered peer: %s (%s) at %s:%d", peer.Name, peer.ID, peer.Addr, peer.Port)

	// Create connected status
	status := &protocol.PeerStatus{
		Peer:      peer,
		Connected: true,
		LastSeen:  peer.LastSeen,
	}

	// Forward to application
	select {
	case m.connectionStatus <- status:
	default:
		log.Printf("Connection status buffer full, dropping discovery notification")
	}
}

// periodicMaintenance handles cleanup and maintenance tasks
func (m *meshImpl) periodicMaintenance() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.performMaintenance()
		}
	}
}

func (m *meshImpl) performMaintenance() {
	// Clean up stale peers
	m.mutex.Lock()
	now := time.Now()
	staleTimeout := time.Duration(m.config.PeerTimeoutSeconds) * time.Second

	for id, peer := range m.peers {
		if now.Sub(peer.LastSeen) > staleTimeout {
			log.Printf("Removing stale peer: %s", id)
			delete(m.peers, id)
		}
	}
	m.mutex.Unlock()
}

// Utility functions
func (m *meshImpl) sendToTransport(to protocol.NodeID, data []byte) error {
	return m.transport.SendTo(to, data)
}

func (m *meshImpl) broadcastToTransport(data []byte) error {
	return m.transport.Broadcast(data)
}

func (m *meshImpl) isDirectlyConnected(nodeID protocol.NodeID) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	_, exists := m.peers[nodeID]
	return exists
}

func (m *meshImpl) updateStats(updater func(*MessageStats)) {
	m.statsMutex.Lock()
	defer m.statsMutex.Unlock()
	updater(&m.stats)
}

// Interface implementation methods
func (m *meshImpl) GetPeers() map[protocol.NodeID]*protocol.Peer {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[protocol.NodeID]*protocol.Peer)
	for k, v := range m.peers {
		peerCopy := *v
		result[k] = &peerCopy
	}
	return result
}

func (m *meshImpl) GetNetworkTopology() *protocol.NetworkMap {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return &protocol.NetworkMap{
		LocalNode: m.config.LocalNodeID,
		Peers:     m.GetPeers(),
		Routes:    m.aodv.GetRoutingTable(),
		UpdatedAt: time.Now(),
	}
}

func (m *meshImpl) IncomingMessages() <-chan *protocol.Message {
	return m.incomingMessages
}

func (m *meshImpl) ConnectionStatus() <-chan *protocol.PeerStatus {
	return m.connectionStatus
}

func (m *meshImpl) GetLocalNodeID() protocol.NodeID {
	return m.config.LocalNodeID
}

func (m *meshImpl) IsConnected(nodeID protocol.NodeID) bool {
	// Check direct connection
	if m.isDirectlyConnected(nodeID) {
		return true
	}

	// Check if we have a route via AODV
	routes := m.aodv.GetRoutingTable()
	route, exists := routes[nodeID]
	return exists && route.Active && time.Now().Before(route.Lifetime)
}

// Utility functions
func generateMessageID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func getTTLForPriority(priority protocol.Priority) int {
	switch priority {
	case protocol.PriorityEmergency:
		return protocol.EmergencyTTL
	default:
		return protocol.DefaultTTL
	}
}
