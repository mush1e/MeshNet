package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mush1e/meshNet/pkg/mesh"
	"github.com/mush1e/meshNet/pkg/protocol"
	"github.com/mush1e/meshNet/pkg/transport"
	"github.com/mush1e/meshNet/pkg/transport/tcp"
)

func main() {
	// Command line flags
	var (
		nodeID    = flag.String("id", "", "Node ID (will generate if empty)")
		nodeName  = flag.String("name", "", "Human-readable node name")
		port      = flag.Int("port", 8001, "Port to listen on")
		bootstrap = flag.String("bootstrap", "", "Comma-separated list of bootstrap peers (ip:port)")
		testMsg   = flag.Bool("test", false, "Send test messages periodically")
	)
	flag.Parse()

	// Generate node ID if not provided
	if *nodeID == "" {
		*nodeID = generateNodeID()
	}

	// Generate name if not provided
	if *nodeName == "" {
		*nodeName = fmt.Sprintf("Node-%s", (*nodeID)[:8])
	}

	log.Printf("Starting MeshNet node: %s (%s) on port %d", *nodeName, *nodeID, *port)

	// Parse bootstrap peers
	var bootstrapPeers []string
	if *bootstrap != "" {
		bootstrapPeers = strings.Split(*bootstrap, ",")
		log.Printf("Bootstrap peers: %v", bootstrapPeers)
	}

	// Create transport config
	transportConfig := &transport.Config{
		LocalNodeID:   protocol.NodeID(*nodeID),
		LocalNodeName: *nodeName,
		Port:          *port,
	}

	// Create TCP transport
	tcpTransport := tcp.NewTCPTransport(transportConfig, bootstrapPeers)

	// Create mesh config
	meshConfig := mesh.DefaultConfig(protocol.NodeID(*nodeID), *nodeName)

	// Create mesh
	meshNetwork := mesh.NewMesh(meshConfig)

	// Start the mesh
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := meshNetwork.Start(ctx, tcpTransport); err != nil {
		log.Fatalf("Failed to start mesh: %v", err)
	}

	// Start message handlers
	go handleIncomingMessages(meshNetwork)
	go handleConnectionStatus(meshNetwork)

	// Start test message sender if requested
	if *testMsg {
		go sendTestMessages(meshNetwork)
	}

	// Print status
	go printStatus(meshNetwork)

	// Wait for shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Printf("MeshNet node started. Press Ctrl+C to stop.")
	log.Printf("Connected peers will appear below...")

	<-sigChan
	log.Printf("Shutting down...")

	if err := meshNetwork.Stop(); err != nil {
		log.Printf("Error stopping mesh: %v", err)
	}
}

// handleIncomingMessages processes messages received from other nodes
func handleIncomingMessages(mesh mesh.Mesh) {
	for msg := range mesh.IncomingMessages() {
		log.Printf("📨 Received message from %s: %s", msg.From, string(msg.Data))

		// Echo back a response (for testing)
		response := fmt.Sprintf("Echo: %s", string(msg.Data))
		if err := mesh.SendMessage(msg.From, []byte(response), protocol.PriorityNormal); err != nil {
			log.Printf("Failed to send echo response: %v", err)
		}
	}
}

// handleConnectionStatus processes peer connection/disconnection events
func handleConnectionStatus(mesh mesh.Mesh) {
	for status := range mesh.ConnectionStatus() {
		if status.Connected {
			log.Printf("🟢 Peer connected: %s (%s)", status.Peer.Name, status.Peer.ID)
		} else {
			log.Printf("🔴 Peer disconnected: %s (%s)", status.Peer.Name, status.Peer.ID)
		}
	}
}

// sendTestMessages sends periodic test messages to all peers
func sendTestMessages(mesh mesh.Mesh) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	counter := 0
	for range ticker.C {
		counter++
		peers := mesh.GetPeers()

		if len(peers) == 0 {
			log.Printf("📤 No peers to send test message to")
			continue
		}

		// Send to first peer (for testing)
		for peerID := range peers {
			message := fmt.Sprintf("Test message #%d from %s", counter, mesh.GetLocalNodeID())
			if err := mesh.SendMessage(peerID, []byte(message), protocol.PriorityNormal); err != nil {
				log.Printf("Failed to send test message to %s: %v", peerID, err)
			} else {
				log.Printf("📤 Sent test message to %s", peerID)
			}
			break // Only send to first peer
		}
	}
}

// printStatus periodically prints network status
func printStatus(mesh mesh.Mesh) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		peers := mesh.GetPeers()
		topology := mesh.GetNetworkTopology()

		log.Printf("📊 Status: %d peers connected, %d routes known",
			len(peers), len(topology.Routes))

		if len(peers) > 0 {
			log.Printf("   Peers:")
			for id, peer := range peers {
				connected := "❓"
				if mesh.IsConnected(id) {
					connected = "✅"
				}
				log.Printf("     %s %s (%s) at %s:%d",
					connected, peer.Name, peer.ID, peer.Addr, peer.Port)
			}
		}
	}
}

// generateNodeID creates a random node ID
func generateNodeID() string {
	return fmt.Sprintf("node-%d", time.Now().UnixNano()%100000)
}
