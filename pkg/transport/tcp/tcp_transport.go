package tcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/mush1e/meshNet/pkg/protocol"
	"github.com/mush1e/meshNet/pkg/transport"
)

// tcpTransport implements the Transport interface using TCP connections
type tcpTransport struct {
	config *transport.Config

	// Network components
	listener net.Listener
	peers    map[protocol.NodeID]*tcpPeer
	mutex    sync.RWMutex

	// Channels for mesh layer
	peerDiscovered   chan *protocol.Peer
	messageReceived  chan *transport.TransportMessage
	connectionStatus chan *protocol.PeerStatus

	// Control
	ctx    context.Context
	cancel context.CancelFunc

	// Bootstrap peers for testing
	bootstrapPeers []string // Format: "ip:port"
}

// tcpPeer represents a connected peer
type tcpPeer struct {
	peer      *protocol.Peer
	conn      net.Conn
	connected bool
	lastSeen  time.Time
}

// DiscoveryMessage is sent when peers connect to identify themselves
type DiscoveryMessage struct {
	NodeID protocol.NodeID `json:"node_id"`
	Name   string          `json:"name"`
	Port   int             `json:"port"`
	Type   string          `json:"type"` // "discovery"
}

// NewTCPTransport creates a new TCP transport with bootstrap peers for testing
func NewTCPTransport(config *transport.Config, bootstrapPeers []string) transport.Transport {
	return &tcpTransport{
		config:           config,
		peers:            make(map[protocol.NodeID]*tcpPeer),
		peerDiscovered:   make(chan *protocol.Peer, 10),
		messageReceived:  make(chan *transport.TransportMessage, 100),
		connectionStatus: make(chan *protocol.PeerStatus, 10),
		bootstrapPeers:   bootstrapPeers,
	}
}

// Start initializes the TCP transport
func (t *tcpTransport) Start(ctx context.Context) error {
	t.ctx, t.cancel = context.WithCancel(ctx)

	// Start TCP listener
	listenAddr := fmt.Sprintf(":%d", t.config.Port)
	var err error
	t.listener, err = net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("failed to start TCP listener on %s: %v", listenAddr, err)
	}

	log.Printf("TCP transport listening on %s", listenAddr)

	// Start accepting connections
	go t.acceptConnections()

	// Connect to bootstrap peers
	go t.connectToBootstrapPeers()

	// Start maintenance routine
	go t.maintenanceRoutine()

	return nil
}

// Stop shuts down the transport
func (t *tcpTransport) Stop() error {
	if t.cancel != nil {
		t.cancel()
	}

	if t.listener != nil {
		t.listener.Close()
	}

	// Close all peer connections
	t.mutex.Lock()
	for _, peer := range t.peers {
		if peer.conn != nil {
			peer.conn.Close()
		}
	}
	t.mutex.Unlock()

	// Close channels
	close(t.peerDiscovered)
	close(t.messageReceived)
	close(t.connectionStatus)

	return nil
}

// acceptConnections handles incoming TCP connections
func (t *tcpTransport) acceptConnections() {
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			conn, err := t.listener.Accept()
			if err != nil {
				if t.ctx.Err() != nil {
					return // Context cancelled
				}
				log.Printf("Failed to accept connection: %v", err)
				continue
			}

			go t.handleConnection(conn, false) // false = incoming connection
		}
	}
}

// connectToBootstrapPeers connects to predefined bootstrap peers
func (t *tcpTransport) connectToBootstrapPeers() {
	for _, peerAddr := range t.bootstrapPeers {
		go func(addr string) {
			// Add some jitter to avoid connection races
			time.Sleep(time.Duration(len(addr)%5) * time.Second)

			log.Printf("Connecting to bootstrap peer: %s", addr)
			conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
			if err != nil {
				log.Printf("Failed to connect to bootstrap peer %s: %v", addr, err)
				return
			}

			t.handleConnection(conn, true) // true = outgoing connection
		}(peerAddr)
	}
}

// handleConnection manages a TCP connection with a peer
func (t *tcpTransport) handleConnection(conn net.Conn, isOutgoing bool) {
	defer conn.Close()

	// Send our discovery message
	if err := t.sendDiscoveryMessage(conn); err != nil {
		log.Printf("Failed to send discovery message: %v", err)
		return
	}

	// Read messages from this connection
	decoder := json.NewDecoder(conn)

	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			// Set read timeout
			conn.SetReadDeadline(time.Now().Add(30 * time.Second))

			var rawMsg json.RawMessage
			if err := decoder.Decode(&rawMsg); err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue // Timeout is expected
				}
				log.Printf("Connection closed or error reading: %v", err)
				return
			}

			// Try to parse as discovery message first
			var discoveryMsg DiscoveryMessage
			if err := json.Unmarshal(rawMsg, &discoveryMsg); err == nil && discoveryMsg.Type == "discovery" {
				t.handleDiscoveryMessage(&discoveryMsg, conn)
				continue
			}

			// Otherwise, treat as regular message
			t.handleRegularMessage(rawMsg, conn)
		}
	}
}

// sendDiscoveryMessage sends our identity to a peer
func (t *tcpTransport) sendDiscoveryMessage(conn net.Conn) error {
	msg := DiscoveryMessage{
		NodeID: t.config.LocalNodeID,
		Name:   t.config.LocalNodeName,
		Port:   t.config.Port,
		Type:   "discovery",
	}

	encoder := json.NewEncoder(conn)
	return encoder.Encode(msg)
}

// handleDiscoveryMessage processes a discovery message from a peer
func (t *tcpTransport) handleDiscoveryMessage(msg *DiscoveryMessage, conn net.Conn) {
	// Extract peer address
	host, _, _ := net.SplitHostPort(conn.RemoteAddr().String())

	peer := &protocol.Peer{
		ID:       msg.NodeID,
		Name:     msg.Name,
		Addr:     host,
		Port:     msg.Port,
		LastSeen: time.Now(),
	}

	// Don't add ourselves
	if peer.ID == t.config.LocalNodeID {
		return
	}

	// Add to peer list
	t.mutex.Lock()
	tcpPeer := &tcpPeer{
		peer:      peer,
		conn:      conn,
		connected: true,
		lastSeen:  time.Now(),
	}
	t.peers[peer.ID] = tcpPeer
	t.mutex.Unlock()

	log.Printf("Discovered peer: %s (%s) from %s", peer.Name, peer.ID, conn.RemoteAddr())

	// Notify mesh layer
	select {
	case t.peerDiscovered <- peer:
	case <-t.ctx.Done():
		return
	}

	// Send connection status
	status := &protocol.PeerStatus{
		Peer:      peer,
		Connected: true,
		LastSeen:  peer.LastSeen,
	}

	select {
	case t.connectionStatus <- status:
	case <-t.ctx.Done():
		return
	}
}

// handleRegularMessage processes a regular message from a peer
func (t *tcpTransport) handleRegularMessage(rawMsg json.RawMessage, conn net.Conn) {
	// Find which peer this connection belongs to
	var fromPeer protocol.NodeID

	t.mutex.RLock()
	for id, peer := range t.peers {
		if peer.conn == conn {
			fromPeer = id
			peer.lastSeen = time.Now()
			break
		}
	}
	t.mutex.RUnlock()

	if fromPeer == "" {
		log.Printf("Received message from unknown peer")
		return
	}

	// Create transport message
	transportMsg := &transport.TransportMessage{
		Data:       []byte(rawMsg),
		From:       fromPeer,
		ReceivedAt: time.Now().Unix(),
		Metadata:   map[string]interface{}{"transport": "tcp"},
	}

	// Send to mesh layer
	select {
	case t.messageReceived <- transportMsg:
	case <-t.ctx.Done():
		return
	default:
		log.Printf("Message channel full, dropping message from %s", fromPeer)
	}
}

// SendTo sends a message to a specific peer
func (t *tcpTransport) SendTo(peerID protocol.NodeID, data []byte) error {
	t.mutex.RLock()
	peer, exists := t.peers[peerID]
	t.mutex.RUnlock()

	if !exists || !peer.connected {
		return fmt.Errorf("peer %s not connected", peerID)
	}

	// Send raw JSON data
	encoder := json.NewEncoder(peer.conn)
	var rawMsg json.RawMessage = data
	return encoder.Encode(rawMsg)
}

// Broadcast sends a message to all connected peers
func (t *tcpTransport) Broadcast(data []byte) error {
	t.mutex.RLock()
	peers := make([]*tcpPeer, 0, len(t.peers))
	for _, peer := range t.peers {
		if peer.connected {
			peers = append(peers, peer)
		}
	}
	t.mutex.RUnlock()

	var lastErr error
	for _, peer := range peers {
		encoder := json.NewEncoder(peer.conn)
		var rawMsg json.RawMessage = data
		if err := encoder.Encode(rawMsg); err != nil {
			log.Printf("Failed to broadcast to %s: %v", peer.peer.ID, err)
			lastErr = err
		}
	}

	return lastErr
}

// DiscoverPeers actively discovers new peers (for TCP, we try bootstrap peers)
func (t *tcpTransport) DiscoverPeers() error {
	go t.connectToBootstrapPeers()
	return nil
}

// Interface methods
func (t *tcpTransport) PeerDiscovered() <-chan *protocol.Peer {
	return t.peerDiscovered
}

func (t *tcpTransport) MessageReceived() <-chan *transport.TransportMessage {
	return t.messageReceived
}

func (t *tcpTransport) ConnectionStatusChanged() <-chan *protocol.PeerStatus {
	return t.connectionStatus
}

func (t *tcpTransport) GetConnectedPeers() map[protocol.NodeID]*protocol.Peer {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	result := make(map[protocol.NodeID]*protocol.Peer)
	for id, peer := range t.peers {
		if peer.connected {
			result[id] = peer.peer
		}
	}
	return result
}

// maintenanceRoutine handles periodic maintenance
func (t *tcpTransport) maintenanceRoutine() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			t.cleanupStalePeers()
		}
	}
}

// cleanupStalePeers removes peers that haven't been seen recently
func (t *tcpTransport) cleanupStalePeers() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	now := time.Now()
	timeout := 60 * time.Second

	for id, peer := range t.peers {
		if now.Sub(peer.lastSeen) > timeout {
			log.Printf("Removing stale peer: %s", id)
			if peer.conn != nil {
				peer.conn.Close()
			}

			// Send disconnection status
			status := &protocol.PeerStatus{
				Peer:      peer.peer,
				Connected: false,
				LastSeen:  peer.lastSeen,
				Error:     "connection timeout",
			}

			select {
			case t.connectionStatus <- status:
			default:
			}

			delete(t.peers, id)
		}
	}
}
