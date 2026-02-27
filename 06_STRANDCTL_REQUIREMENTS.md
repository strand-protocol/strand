# StrandCtl — Command-Line Interface & Management Tool

## Module: `strandctl/`

## Language: Go (1.22+)

## Binary: `strandctl`

---

## 1. Overview

StrandCtl is the operator-facing CLI tool for the Strand Protocol stack. It provides a unified interface for deploying, configuring, monitoring, diagnosing, and managing all Strand components across a fleet of switches, NICs, and servers. StrandCtl is to Strand what `kubectl` is to Kubernetes — the single pane of glass for infrastructure operators.

StrandCtl communicates with Strand nodes via the StrandAPI control plane and can also directly interface with StrandLink/StrandRoute firmware via SSH and vendor-specific management APIs.

---

## 2. References

| Reference | Relevance |
|-----------|-----------|
| `kubectl` (Kubernetes CLI) | Primary UX reference — StrandCtl follows similar command structure, output formatting, and resource-based interaction model |
| `calicoctl` (Calico CNI CLI) | Reference for network policy management CLIs |
| `sonic-cli` (SONiC CLI) | Reference for switch OS CLI patterns |
| `istioctl` (Istio service mesh CLI) | Reference for mesh diagnostics, proxy injection, and config validation |
| `ethtool` (Linux NIC tool) | Reference for NIC-level diagnostics that StrandCtl wraps |
| Cobra (Go CLI framework) | CLI framework used for implementation |

---

## 3. Command Structure

### 3.1 Top-Level Commands

```
strandctl
├── node                   # Manage Strand nodes
│   ├── list               # List all known nodes in the network
│   ├── info <node-id>     # Show detailed info for a node
│   ├── join <address>     # Join a node to the Strand network
│   ├── remove <node-id>   # Remove a node from the network
│   └── label <node-id>    # Add/remove labels on a node
│
├── firmware               # Firmware management
│   ├── status <node-id>   # Show current firmware version on a node
│   ├── flash <node-id>    # Flash StrandLink firmware to a NIC
│   ├── upgrade <node-id>  # Rolling firmware upgrade
│   ├── rollback <node-id> # Rollback to previous firmware
│   └── list-supported     # List supported NIC models and firmware versions
│
├── route                  # StrandRoute management
│   ├── table              # Show the routing table (all capabilities)
│   ├── resolve <sad>      # Resolve a SAD query to matching nodes
│   ├── advertise          # Manually advertise a capability
│   ├── withdraw <node-id> # Withdraw a capability advertisement
│   ├── peers              # Show gossip peer membership
│   └── policy             # Manage routing policies
│       ├── list
│       ├── apply <file>
│       └── delete <name>
│
├── stream                 # StrandStream management
│   ├── list               # List active connections and streams
│   ├── info <conn-id>     # Show connection/stream details
│   ├── close <conn-id>    # Force-close a connection
│   └── stats              # Aggregate transport statistics
│
├── trust                  # StrandTrust management
│   ├── ca                 # Certificate Authority operations
│   │   ├── init           # Initialize a new root CA
│   │   ├── issue          # Issue a new MIC
│   │   ├── revoke <serial># Revoke a MIC
│   │   └── list           # List issued MICs
│   ├── mic                # MIC operations
│   │   ├── generate       # Generate a new node identity keypair + self-signed MIC
│   │   ├── show <file>    # Display MIC contents
│   │   ├── verify <file>  # Verify MIC signature chain
│   │   └── export <file>  # Export MIC in various formats
│   ├── keys               # Key management
│   │   ├── generate       # Generate Ed25519 keypair
│   │   ├── show           # Show public key and Node ID
│   │   └── rotate         # Rotate node keys (re-issue MIC)
│   └── log                # Transparency log operations
│       ├── query           
│       └── verify          
│
├── diagnose               # Network diagnostics
│   ├── ping <node-id>     # StrandStream-level ping (latency measurement)
│   ├── traceroute <node-id># Trace the StrandRoute path to a node
│   ├── mtu <node-id>      # Test path MTU between local and remote
│   ├── bandwidth <node-id># Bandwidth test (iperf-like for StrandStream)
│   ├── capture             # Packet capture (tcpdump-like for StrandLink frames)
│   │   ├── start
│   │   ├── stop
│   │   └── read <file>
│   └── health             # Run health checks across all nodes
│
├── config                 # Configuration management
│   ├── show               # Show current strandctl configuration
│   ├── set <key> <value>  # Set configuration value
│   ├── validate <file>    # Validate a configuration file
│   └── apply <file>       # Apply configuration to one or more nodes
│
├── metrics                # Observability
│   ├── dashboard          # Real-time terminal dashboard (TUI)
│   ├── export             # Export metrics in Prometheus/JSON format
│   └── top                # Top-like view of busiest nodes/streams
│
└── version                # Show strandctl and protocol versions
```

---

## 4. Architecture & Components

### 4.1 Source Tree Structure

```
strandctl/
├── go.mod
├── go.sum
├── main.go                           # Entry point
├── cmd/
│   ├── root.go                       # Root command, global flags
│   ├── node.go                       # node subcommands
│   ├── firmware.go                   # firmware subcommands
│   ├── route.go                      # route subcommands
│   ├── stream.go                     # stream subcommands
│   ├── trust.go                      # trust subcommands
│   ├── diagnose.go                   # diagnose subcommands
│   ├── config.go                     # config subcommands
│   ├── metrics.go                    # metrics subcommands
│   └── version.go                    # version command
├── pkg/
│   ├── api/
│   │   ├── client.go                 # Control plane API client
│   │   ├── node_client.go            # Node management API
│   │   ├── route_client.go           # Routing table API
│   │   ├── trust_client.go           # StrandTrust CA API
│   │   └── firmware_client.go        # Firmware management API
│   ├── firmware/
│   │   ├── flasher.go                # Firmware flashing engine
│   │   ├── ssh.go                    # SSH transport for remote firmware operations
│   │   ├── supported_nics.go         # Registry of supported NIC models
│   │   └── images/                   # Firmware image handling
│   ├── capture/
│   │   ├── capture.go                # StrandLink frame capture engine
│   │   ├── filter.go                 # BPF-like capture filter expressions
│   │   └── writer.go                 # pcap-ng writer for captured frames
│   ├── tui/
│   │   ├── dashboard.go              # Terminal UI dashboard (bubbletea)
│   │   ├── top.go                    # Top-like stream view
│   │   └── styles.go                 # TUI styling
│   ├── output/
│   │   ├── table.go                  # Tabular output formatter
│   │   ├── json.go                   # JSON output formatter
│   │   ├── yaml.go                   # YAML output formatter
│   │   └── wide.go                   # Wide table output
│   └── config/
│       ├── config.go                 # strandctl config file management
│       └── context.go                # Multi-cluster context switching
├── tests/
│   ├── cmd_test.go                   # Command parsing tests
│   ├── api_client_test.go            # API client tests (mock server)
│   ├── firmware_test.go              # Firmware flash tests (mock SSH)
│   ├── capture_test.go               # Frame capture tests
│   └── e2e/
│       └── e2e_test.go               # End-to-end CLI tests
└── docs/
    ├── strandctl.md                     # Complete CLI reference
    └── examples.md                   # Usage examples
```

---

## 5. Functional Requirements

### 5.1 Core CLI

| ID | Requirement | Priority |
|----|-------------|----------|
| NC-CLI-001 | All commands support `--output` flag: `table` (default), `json`, `yaml`, `wide` | P0 |
| NC-CLI-002 | All commands support `--context` flag to select target cluster/network | P0 |
| NC-CLI-003 | Configuration file at `~/.strand/config.yaml` for defaults, API endpoints, credentials | P0 |
| NC-CLI-004 | Tab completion for bash, zsh, fish, powershell | P1 |
| NC-CLI-005 | `--verbose` / `-v` flag for debug output on all commands | P0 |
| NC-CLI-006 | `--dry-run` flag for destructive operations (firmware flash, node remove, route withdraw) | P0 |
| NC-CLI-007 | Interactive confirmation for destructive operations (unless `--yes` flag provided) | P0 |
| NC-CLI-008 | Colorized terminal output with `--no-color` flag for CI/piping | P1 |

### 5.2 Node Management

| ID | Requirement | Priority |
|----|-------------|----------|
| NC-NODE-001 | `node list`: display all nodes with Node ID (truncated), hostname, IP, status, firmware version, StrandTrust level | P0 |
| NC-NODE-002 | `node info`: detailed node information including all capabilities, active connections, metrics | P0 |
| NC-NODE-003 | `node join`: connect to a remote node's control plane and join it to the local Strand network | P0 |

### 5.3 Firmware Management

| ID | Requirement | Priority |
|----|-------------|----------|
| NC-FW-001 | `firmware flash`: push StrandLink firmware to target NIC via SSH + vendor tools. Support ConnectX, E810, BlueField | P0 |
| NC-FW-002 | `firmware status`: query running firmware version, NIC model, driver version | P0 |
| NC-FW-003 | `firmware upgrade`: rolling upgrade across fleet with configurable parallelism and health checks between batches | P1 |
| NC-FW-004 | `firmware rollback`: restore previous firmware version from backup | P1 |
| NC-FW-005 | Pre-flash validation: check NIC model, driver compatibility, free space before attempting flash | P0 |

### 5.4 Diagnostics

| ID | Requirement | Priority |
|----|-------------|----------|
| NC-DIAG-001 | `diagnose ping`: send StrandStream PING frames, report min/avg/max/p99 RTT | P0 |
| NC-DIAG-002 | `diagnose traceroute`: discover StrandRoute path hops to destination node | P0 |
| NC-DIAG-003 | `diagnose bandwidth`: bidirectional throughput test using StrandStream BE mode | P1 |
| NC-DIAG-004 | `diagnose capture`: capture StrandLink frames with BPF-like filter syntax, write to pcap-ng | P1 |
| NC-DIAG-005 | `diagnose health`: run health checks on all reachable nodes, report degraded/failed nodes | P0 |

### 5.5 Metrics & TUI

| ID | Requirement | Priority |
|----|-------------|----------|
| NC-MET-001 | `metrics dashboard`: real-time TUI showing node count, total streams, aggregate throughput, top talkers | P1 |
| NC-MET-002 | `metrics top`: top-like view sorted by bandwidth/connections/latency | P1 |
| NC-MET-003 | `metrics export`: export current metrics as JSON or Prometheus exposition format | P0 |

---

## 6. Non-Functional Requirements

| ID | Requirement | Target |
|----|-------------|--------|
| NC-NF-001 | Command execution latency (simple queries) | < 500ms including API round-trip |
| NC-NF-002 | Binary size | < 30MB (single static binary) |
| NC-NF-003 | Cross-platform | Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64) |
| NC-NF-004 | Zero runtime dependencies | Statically linked, no external tools required |

---

## 7. Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| `github.com/spf13/cobra` | 1.8+ | CLI framework |
| `github.com/spf13/viper` | 1.18+ | Configuration management |
| `github.com/charmbracelet/bubbletea` | 0.25+ | Terminal UI framework |
| `github.com/charmbracelet/lipgloss` | 0.9+ | TUI styling |
| `github.com/olekukonez/tablewriter` | 0.0.5+ | Table output formatting |
| `golang.org/x/crypto/ssh` | Latest | SSH client for firmware operations |
| `github.com/fatih/color` | 1.16+ | Colorized output |
| StrandAPI client SDK | — | Control plane communication |
