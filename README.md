# MeshNet

**The internet without the internet.**

> ⚠️ **Early Development** - This project is in active development. Core functionality is being built. Not ready for production use.

MeshNet is a decentralized communication network that turns any collection of devices into an intelligent mesh. When cell towers fail, WiFi dies, or you're beyond the reach of traditional infrastructure, MeshNet automatically creates resilient peer-to-peer networks that route messages through multiple devices to reach their destination.

Built in Go for maximum performance and reliability, MeshNet implements advanced routing algorithms that adapt to changing network conditions, automatically finding the best path for your data even as devices move, disconnect, or fail.

**Zero servers. Zero infrastructure. Zero single points of failure.**

## Vision

```bash
# Future usage (not yet implemented)
meshnet join                    # Auto-discover and join network
meshnet send @alice "Hey!"      # Send message to specific peer
meshnet broadcast "Anyone copy?" # Broadcast to all reachable peers
meshnet share document.pdf      # Share files across the mesh
meshnet topology                # Visualize network structure
```

## Goals

- **Infrastructure-Free** - Works without internet, cellular, or centralized servers
- **Self-Healing** - Automatically routes around failed connections
- **Cross-Platform** - Linux, macOS, Windows, and ARM devices
- **Developer-Friendly** - Simple CLI interface and REST API
- **Secure** - End-to-end encryption with perfect forward secrecy
- **Scalable** - Supports networks from 2 to 1000+ devices

## Planned Features

### Core Networking
- [x] Project planning and architecture
- [ ] Basic TCP peer-to-peer communication
- [ ] UDP multicast peer discovery
- [ ] Multi-hop message routing (AODV protocol)
- [ ] Connection management and failover
- [ ] Network topology visualization

### Security & Encryption
- [ ] X25519 key exchange
- [ ] AES-256-GCM message encryption
- [ ] Ed25519 node authentication
- [ ] Perfect forward secrecy

### User Interface
- [ ] CLI interface with Cobra
- [ ] Configuration management
- [ ] REST API for integration
- [ ] Real-time status monitoring

### Advanced Features
- [ ] File sharing and chunked transfers
- [ ] Group channels and messaging
- [ ] Network bridging capabilities
- [ ] Quality of service controls

## Architecture (Planned)

```
┌─────────────────────────────────────┐
│           Application Layer         │
│  (CLI, REST API, Message Handlers)  │
├─────────────────────────────────────┤
│           Protocol Layer            │
│   (Message Routing, Encryption)     │
├─────────────────────────────────────┤
│          Transport Layer            │
│      (TCP Connections, UDP)         │
├─────────────────────────────────────┤
│          Discovery Layer            │
│   (mDNS, WiFi Scan, Bluetooth)      │
└─────────────────────────────────────┘
```

## Getting Started (When Ready)

### Prerequisites
- Go 1.21 or higher
- Network interface (WiFi/Ethernet)
- Linux, macOS, or Windows

### Installation (Future)
```bash
# Install from source
git clone https://github.com/your-username/meshnet.git
cd meshnet
go build -o meshnet cmd/meshnet/main.go

# Or download binary releases (when available)
# Coming soon: package manager support
```

## Use Cases

### Emergency Response
- Coordinate disaster relief when infrastructure fails
- Search and rescue communication in remote areas
- Emergency shelter coordination

### Development & Testing
- Distributed system development
- Network protocol research
- Offline-first application testing

### Remote Areas
- Communication in areas without cell coverage
- Rural community networks
- Off-grid installations

## Contributing

This project is just getting started! Contributions are welcome:

- **Code**: Help implement the core networking components
- **Documentation**: Improve docs and examples
- **Testing**: Write tests and find bugs
- **Ideas**: Suggest features and improvements

See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

### Current Focus
The project is in early development. Most valuable contributions right now:
1. Core networking implementation (Phase 1-2)
2. Protocol design feedback
3. Cross-platform testing
4. Documentation improvements

## Connect

- **Issues**: Report bugs or request features
- **Discussions**: Ask questions or share ideas
- **Pull Requests**: Contribute code or documentation

## License

License - see [LICENSE](LICENSE) for details.

---

⭐ **Star this repo** if you're interested in decentralized networking and want to follow development progress!
