# Contributing to MeshNet

Thank you for your interest in contributing to MeshNet! This project aims to create a robust, infrastructure-free mesh networking solution for emergency communications and remote area connectivity.

## Quick Start

1. **Fork** the repository on GitHub
2. **Clone** your fork locally
3. **Create** a feature branch from `main`
4. **Make** your changes
5. **Test** your changes thoroughly
6. **Submit** a pull request

## Development Setup

### Prerequisites

- Go 1.21 or higher
- Git
- A modern text editor or IDE
- Linux, macOS, or Windows development environment

### Local Development

```bash
# Clone your fork
git clone https://github.com/YOUR-USERNAME/meshnet.git
cd meshnet

# Install dependencies (when available)
go mod download

# Run tests (when implemented)
go test ./...

# Build the project
go build -o meshnet cmd/meshnet/main.go
```

## Current Development Priorities

MeshNet is in early development. Here's where we need the most help:

### Phase 1: Foundation (High Priority)
- [ ] Basic TCP server/client implementation in `pkg/transport/`
- [ ] UDP multicast peer discovery
- [ ] Message serialization and protocol design
- [ ] Connection management and error handling
- [ ] Unit tests for core components

### Phase 2: Multi-Peer Network (Medium Priority)
- [ ] Connection pool management
- [ ] Message broadcasting with loop prevention
- [ ] Basic network topology tracking
- [ ] Peer lifecycle management

### Phase 3: Smart Routing (Future)
- [ ] AODV routing protocol implementation
- [ ] Route discovery and maintenance
- [ ] Path optimization algorithms

## What We're Looking For

### Code Contributions
- **Transport Layer**: TCP/UDP networking, peer discovery, connection management
- **Protocol Layer**: Message formats, routing algorithms, error handling
- **Testing**: Unit tests, integration tests, network simulation
- **Documentation**: Code comments, API docs, examples
- **Cross-Platform**: Windows, macOS, Linux compatibility testing

### Non-Code Contributions
- **Documentation**: README improvements, tutorials, architecture docs
- **Design**: Protocol design, security architecture, API design
- **Testing**: Manual testing, edge case discovery, performance testing
- **Community**: Issue triage, question answering, onboarding new contributors

## Development Guidelines

### Code Style

We follow standard Go conventions:

```bash
# Format your code
go fmt ./...

# Run the linter (when configured)
golangci-lint run

# Check for common issues
go vet ./...
```

### Code Organization

```
pkg/
├── transport/     # Transport layer (TCP, UDP, discovery)
├── protocol/      # Message formats and routing protocols  
├── mesh/         # High-level mesh networking API
└── security/     # Encryption and authentication (future)

cmd/
└── meshnet/      # CLI application

internal/         # Internal packages (when needed)
docs/            # Documentation
examples/        # Example usage
```

### Commit Messages

Use clear, descriptive commit messages:

```
feat: add TCP transport implementation
fix: resolve connection timeout in peer discovery  
docs: update README with installation instructions
test: add unit tests for AODV route table
```

### Testing

- Write unit tests for all new functions
- Add integration tests for component interactions
- Test cross-platform compatibility when possible
- Include edge cases and error conditions

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./pkg/transport/
```

## Bug Reports

When reporting bugs, please include:

1. **Environment**: OS, Go version, hardware details
2. **Steps to reproduce**: Exact steps that trigger the bug
3. **Expected behavior**: What should happen
4. **Actual behavior**: What actually happens
5. **Logs/Output**: Any relevant log messages or error output
6. **Network setup**: How many nodes, connection types, etc.

Use our bug report template:

```markdown
**Environment:**
- OS: [e.g., Ubuntu 22.04]
- Go version: [e.g., 1.21.0]
- MeshNet version: [e.g., commit hash]

**Description:**
Brief description of the issue

**Steps to reproduce:**
1. Start meshnet with...
2. Connect second node...
3. Send message...

**Expected behavior:**
Message should be delivered

**Actual behavior:**
Message times out after 30s

**Logs:**
[Include relevant log output]
```

## Feature Requests

We welcome new ideas! Before submitting a feature request:

1. **Search existing issues** to avoid duplicates
2. **Consider the scope** - does it fit the project goals?
3. **Think about implementation** - is it technically feasible?
4. **Consider emergency use cases** - how does it help in crisis situations?

Use our feature request template:

```markdown
**Problem:**
What problem does this solve?

**Proposed solution:**
How should this work?

**Alternatives considered:**
What other approaches did you consider?

**Emergency relevance:**
How does this help in emergency/disaster scenarios?
```

## Security

Since MeshNet is designed for emergency communications, security is critical:

- **Don't submit security vulnerabilities as public issues**
- **Email security concerns** to [your-email] (when established)
- **Consider threat models** - what attacks should we defend against?
- **Think about emergency scenarios** - how might attackers exploit disasters?

## Documentation

Documentation is as important as code:

- **Keep README.md updated** with new features
- **Comment your code** - especially complex algorithms
- **Write examples** - show how to use new features
- **Update architecture docs** - reflect design changes

## Code Review Process

1. **All contributions go through pull requests**
2. **At least one maintainer review** required
3. **CI tests must pass** (when implemented)
4. **Address review feedback** before merging
5. **Squash commits** for clean history

### What We Look For

- **Correctness**: Does the code work as intended?
- **Performance**: Is it efficient for mesh networking?
- **Security**: Are there potential vulnerabilities?
- **Maintainability**: Is the code clear and well-structured?
- **Testing**: Are there adequate tests?
- **Documentation**: Is it well-documented?

## Release Process

(Future - when we have releases)

1. Version using semantic versioning (v0.1.0, v0.2.0, etc.)
2. Tag releases in Git
3. Generate release notes
4. Build cross-platform binaries
5. Update package manager configs

## Recognition

Contributors will be:
- **Listed in README.md** contributors section
- **Mentioned in release notes** for significant contributions
- **Invited to maintainer team** for sustained contributions

## Getting Help

- **GitHub Discussions**: Ask questions and share ideas
- **GitHub Issues**: Report bugs and request features
- **Code Review**: Get feedback on your contributions

## Checklist for Contributors

Before submitting a pull request:

- [ ] Code follows Go conventions (`go fmt`, `go vet`)
- [ ] Tests pass (`go test ./...`)
- [ ] Documentation updated (README, code comments)
- [ ] Changes work on multiple platforms (if applicable)
- [ ] Security implications considered
- [ ] Emergency use cases considered
- [ ] Pull request description explains the change

## Project Vision

Remember our core goals when contributing:

- **Infrastructure-Free**: Works without internet/cellular
- **Self-Healing**: Automatically routes around failures  
- **Cross-Platform**: Runs everywhere
- **Developer-Friendly**: Simple to use and integrate
- **Secure**: Protects communications from attackers
- **Emergency-Ready**: Reliable when infrastructure fails

## License

By contributing to MeshNet, you agree that your contributions will be licensed under the GNU General Public License v3.0.

---

**Ready to contribute?** Check out our [good first issues](https://github.com/your-username/meshnet/labels/good%20first%20issue) or join the discussion about the project roadmap!

Let's build the internet without the internet!