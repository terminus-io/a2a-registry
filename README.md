# A2A Registry

[![Go Version](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-v1.27+-326CE5?style=flat&logo=kubernetes)](https://kubernetes.io/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

A **Kubernetes-native Agent Registry** for the [Agent-to-Agent (A2A) protocol](https://github.com/google/A2A) — an open standard by Google for enabling direct agent-to-agent communication.

Built with the [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) framework, a2a-registry extends Kubernetes with Custom Resource Definitions (CRDs) to manage agent registration, discovery, health checking, and search — all within your cluster.

---

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Quick Start](#quick-start)
- [Installation](#installation)
- [Usage](#usage)
  - [Creating a Registry](#creating-a-registry)
  - [Registering an Agent](#registering-an-agent)
  - [Discovering Agents via the API](#discovering-agents-via-the-api)
- [API Reference](#api-reference)
- [Configuration](#configuration)
- [Development](#development)
- [Examples](#examples)
- [License](#license)

---

## Features

- **Kubernetes-native** — Agents are defined as CRDs (`A2AAgent`, `A2ARegistry`). No external database needed; Kubernetes is the source of truth.
- **Automatic Health Checking** — Periodically fetches each agent's [Agent Card](https://github.com/google/A2A/blob/main/specification/a2a.json) from `/.well-known/agent-card.json` and tracks lifecycle phases: `Pending → Ready → Error → Unreachable`.
- **Built-in Discovery API** — A RESTful HTTP API server (port `8082`) runs inside the operator, exposing endpoints for agent listing, search, and card retrieval.
- **Flexible Discovery Scope** — Configure discovery at `Cluster` or `Namespace` level, with optional label selectors and namespace filters.
- **Admission Webhooks** — Validating and mutating webhooks ensure agent specs are correct (URL format, unique skill IDs, default health check settings).
- **Multi-Architecture** — Pre-built Docker images support `linux/amd64` and `linux/arm64`.
- **Prometheus Metrics** — Exposes metrics on port `8080` for monitoring.
- **Leader Election** — Optional leader election for HA deployments.

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                  Kubernetes Cluster                  │
│                                                      │
│  ┌──────────────────────────────────────────────┐   │
│  │           a2a-registry (Operator)             │   │
│  │                                               │   │
│  │  ┌─────────────┐  ┌──────────────┐           │   │
│  │  │  A2AAgent   │  │ A2ARegistry  │           │   │
│  │  │ Controller  │  │  Controller  │           │   │
│  │  └──────┬──────┘  └──────┬───────┘           │   │
│  │         │                │                    │   │
│  │         ▼                ▼                    │   │
│  │  ┌─────────────┐  ┌──────────────┐           │   │
│  │  │   Health    │  │   Registry   │           │   │
│  │  │   Checker   │  │   API Server │           │   │
│  │  └──────┬──────┘  │   (:8082)    │           │   │
│  │         │         └──────┬───────┘           │   │
│  └─────────┼────────────────┼───────────────────┘   │
│            │                │                        │
│            ▼                ▼                        │
│  ┌──────────────────┐  ┌──────────────────┐         │
│  │   Agent Pod A    │  │  External Client  │        │
│  │  /.well-known/   │  │  GET /api/v1/     │        │
│  │  agent-card.json │  │  agents           │        │
│  └──────────────────┘  └──────────────────┘         │
│                                                      │
│  ┌──────────────────┐                               │
│  │   Agent Pod B    │                               │
│  │  /.well-known/   │                               │
│  │  agent-card.json │                               │
│  └──────────────────┘                               │
└─────────────────────────────────────────────────────┘
```

**Key components:**

| Component | Description |
|---|---|
| **A2AAgent Controller** | Watches `A2AAgent` CRs, performs health checks by fetching agent cards, updates status conditions and phase. |
| **A2ARegistry Controller** | Watches `A2ARegistry` CRs, counts registered/healthy agents, pushes discovery configuration to the API server. |
| **Registry API Server** | HTTP server that serves the registry's own Agent Card and provides agent discovery/search endpoints. Runs on all replicas (no leader election required). |
| **Agent Card Resolver** | Fetches and parses agent cards from `/.well-known/agent-card.json` (with fallback to `/.well-known/agent.json`). |
| **Admission Webhooks** | Validate agent specs on create/update, and set default values for protocol version and health check config. |

---

## Quick Start

### Prerequisites

- Go 1.25+
- A running Kubernetes cluster (v1.27+)
- `kubectl` configured with cluster access
- `kustomize` (for deployment)

### 1. Install the CRDs and deploy the operator

```bash
# Clone the repository
git clone https://github.com/terminus-io/a2a-registry.git
cd a2a-registry

# Install CRDs
make install

# Deploy the operator
make deploy
```

### 2. Create a registry

```bash
kubectl apply -f config/samples/a2a.io_v1_a2aregistry.yaml
```

### 3. Register an agent

```bash
kubectl apply -f config/samples/a2a.io_v1_a2aagent.yaml
```

### 4. Query the registry API

```bash
# List all agents
kubectl port-forward -n a2a-registry-system svc/a2a-registry-controller-manager 8082:8082
curl http://localhost:8082/api/v1/agents

# Search agents
curl "http://localhost:8082/api/v1/search?q=hello&tag=demo"
```

---

## Installation

### From source

```bash
make build          # builds bin/manager
make docker-build   # builds the Docker image
make docker-push    # pushes to your registry
```

### Using pre-built images

```bash
# Set your image repository
export IMG=your-registry/a2a-registry:latest

make docker-build
make docker-push
make deploy
```

### Multi-arch build

```bash
make docker-buildx PLATFORMS=linux/amd64,linux/arm64
```

### Uninstall

```bash
make undeploy       # removes the operator deployment
make uninstall      # removes CRDs
```

---

## Usage

### Creating a Registry

The `A2ARegistry` resource is cluster-scoped and defines global settings for agent discovery, registration policies, and the API server.

```yaml
apiVersion: a2a.io/v1
kind: A2ARegistry
metadata:
  name: default-registry
spec:
  discovery:
    scope: "Cluster"           # "Cluster" or "Namespace"
    # labelSelector: "app=my-agent"
    # namespaces: ["ns1", "ns2"]
  registration:
    requireApproval: false
    requireHealthCheck: true
    requireCardMatch: false
  healthCheck:
    intervalSeconds: 120
    timeoutSeconds: 15
  apiServer:
    port: 8082
    bindAddress: "0.0.0.0"
```

### Registering an Agent

An `A2AAgent` resource represents a single A2A agent in the cluster. It is namespaced.

```yaml
apiVersion: a2a.io/v1
kind: A2AAgent
metadata:
  name: hello-world-agent
  namespace: default
spec:
  name: "Hello World Agent"
  description: "A simple example agent that returns greetings"
  version: "1.0.0"
  url: "http://agent-service.default.svc.cluster.local:9001"
  capabilities:
    streaming: true
    pushNotifications: false
  skills:
    - id: "hello_world"
      name: "Hello, world!"
      description: "Returns a greeting message"
      tags:
        - "greeting"
        - "demo"
      examples:
        - "hi"
        - "hello"
  defaultInputModes:
    - "text"
  defaultOutputModes:
    - "text"
  protocolVersion: "1.0"
  tags:
    - "demo"
    - "example"
  enabled: true
  healthCheck:
    intervalSeconds: 60
    timeoutSeconds: 10
    failureThreshold: 3
```

**Key fields:**

| Field | Description |
|---|---|
| `spec.url` | Base endpoint URL of the agent (must be `http://` or `https://`) |
| `spec.skills` | List of skills the agent can perform, each with a unique `id` |
| `spec.enabled` | Set to `false` to disable health checking and remove from discovery |
| `spec.capabilities` | Declare `streaming` and/or `pushNotifications` support |
| `spec.healthCheck` | Per-agent health check override (defaults: interval=60s, timeout=10s, failureThreshold=3) |
| `spec.tags` | Arbitrary tags for searching and filtering |

**Status fields:**

| Field | Description |
|---|---|
| `status.phase` | Lifecycle phase: `Pending`, `Ready`, `Error`, `Unreachable` |
| `status.health` | Health: `Healthy`, `Unhealthy`, `Unknown` |
| `status.lastHeartbeat` | Timestamp of the last successful health check |
| `status.agentCardHash` | SHA256 hash of the fetched agent card |
| `status.conditions` | Kubernetes standard conditions |

### Discovering Agents via the API

The registry API server exposes the following endpoints:

#### List agents

```bash
GET /api/v1/agents
GET /api/v1/agents?namespace=default
GET /api/v1/agents?tags=demo,example
GET /api/v1/agents?skill=hello_world
```

#### Get a specific agent

```bash
GET /api/v1/agents/{name}
GET /api/v1/agents/{name}/card    # Returns the A2A Agent Card
```

#### Search agents

```bash
GET /api/v1/search?q=hello                  # Free-text search
GET /api/v1/search?tag=demo                 # Filter by tag
GET /api/v1/search?skill=hello_world        # Filter by skill
GET /api/v1/search?capability=streaming     # Filter by capability
```

#### Registry's own Agent Card

```bash
GET /.well-known/agent-card.json
GET /.well-known/agent.json
```

The registry advertises itself as an agent with "Agent Discovery" and "Agent Search" skills, enabling other agents to discover peers through the registry.

---

## API Reference

### Custom Resource Definitions

| CRD | Scope | Short Name | Description |
|---|---|---|---|
| `A2AAgent` | Namespaced | `a2aa` | Represents a registered A2A agent |
| `A2ARegistry` | Cluster | `a2areg` | Global registry configuration |

### kubectl Shortcuts

```bash
kubectl get a2aa                    # List all A2AAgents
kubectl get a2areg                  # List all A2ARegistries
kubectl describe a2aa <name>        # Describe an agent
kubectl get a2aa -o wide            # Show additional columns (Phase, Health, URL)
```

### Agent Lifecycle

```
  [Create] ──► Pending ──► Ready ◄── (health check passes)
                  │            │
                  ▼            ▼
                Error    Unreachable
                  │            │
                  └─────┬──────┘
                        ▼
                      Ready (after recovery)
```

---

## Configuration

### Operator Flags

| Flag | Default | Description |
|---|---|---|
| `--metrics-bind-address` | `:8080` | Prometheus metrics endpoint |
| `--health-probe-bind-address` | `:8081` | Health/readiness probe endpoint |
| `--registry-api-bind-address` | `:8082` | Registry discovery API |
| `--leader-elect` | `false` | Enable leader election |
| `--leader-election-id` | `a2a-registry` | Leader election identity |

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `ENABLE_WEBHOOKS` | (enabled) | Set to `"false"` to disable admission webhooks |

---

## Development

```bash
# Build the binary
make build

# Run locally (outside cluster)
make run

# Run tests with envtest
make test

# Generate CRD manifests and DeepCopy methods
make generate manifests

# Format and vet
make fmt
make vet

# Build and push Docker image
export IMG=your-registry/a2a-registry:latest
make docker-build
make docker-push
```

### Project structure

```
.
├── api/v1/                    # CRD type definitions + webhooks
├── cmd/manager/               # Operator entry point
├── config/
│   ├── crd/bases/             # Generated CRD YAML manifests
│   ├── rbac/                  # RBAC roles and bindings
│   ├── manager/               # Deployment manifest
│   ├── default/               # Top-level kustomization
│   └── samples/               # Example CRs
├── controllers/               # Reconciliation logic
├── internal/
│   ├── healthcheck/           # Agent health checking
│   └── registry/              # API server + agent card resolver
├── examples/hello-agent/      # Example A2A agent
├── vendor/                    # Vendored dependencies
├── Dockerfile                 # Multi-stage container build
└── Makefile                   # Build & deploy targets
```

---

## Examples

The [`examples/hello-agent/`](examples/hello-agent/) directory contains a fully functional A2A agent implementation:

- Serves an Agent Card at `/.well-known/agent-card.json`
- Accepts JSON-RPC invocations at `/invoke`
- Includes a complete `deploy.yaml` (Namespace, Deployment, Service, and A2AAgent CR)

```bash
cd examples/hello-agent
kubectl apply -f deploy.yaml
```

---

## Related Projects

- [A2A Protocol Specification](https://github.com/google/A2A) — The open standard for agent-to-agent communication
- [a2a-go](https://github.com/a2aproject/a2a-go) — Go SDK for the A2A protocol
- [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) — Framework for building Kubernetes operators

---

## License

This project is licensed under the Apache License 2.0. See [LICENSE](LICENSE) for details.
