# A2A Registry
中文版 | [English Version](README.md) | [日本語版](README_ja.md)

[![Go Version](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-v1.27+-326CE5?style=flat&logo=kubernetes)](https://kubernetes.io/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

基于 Kubernetes 的 **Agent 注册中心**，面向 [Agent-to-Agent (A2A) 协议](https://github.com/google/A2A) —— Google 提出的 agent 间直接通信开放标准。

使用 [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) 框架构建，通过自定义资源（CRD）在集群内管理 agent 的注册、发现、健康检查和搜索。

---

## 目录

- [特性](#特性)
- [架构](#架构)
- [快速开始](#快速开始)
- [安装](#安装)
- [使用指南](#使用指南)
  - [创建注册中心](#创建注册中心)
  - [注册 Agent](#注册-agent)
  - [通过 API 发现 Agent](#通过-api-发现-agent)
  - [Agent 审批流程](#agent-审批流程)
  - [受认证保护的 Agent Card](#受认证保护的-agent-card)
- [API 参考](#api-参考)
- [配置](#配置)
- [监控](#监控)
- [开发](#开发)
- [示例](#示例)
- [许可证](#许可证)

---

## 特性

- **Kubernetes 原生** — Agent 以 CRD（`A2AAgent`、`A2ARegistry`）形式定义。无需外部数据库，Kubernetes 即数据源。
- **HTTP 注册 API** — Agent 可通过 `POST /api/v1/agents` 自行注册，通过 `DELETE /api/v1/agents/{name}` 注销，同时也支持 `kubectl apply`。
- **自动健康检查** — 定期从 `/.well-known/agent-card.json` 拉取每个 agent 的 [Agent Card](https://github.com/google/A2A/blob/main/specification/a2a.json)，追踪生命周期阶段：`Pending → Ready → Error → Unreachable`。
- **可配置失败阈值** — 每个 agent 可设置 `failureThreshold`，控制在连续多少次失败后标记为 `Unreachable`。
- **URL 冲突检测** — 防止重复的 agent URL 污染注册表。冲突的 agent 会被标记为 `Error`。
- **内置发现 API** — 操作器内运行 RESTful HTTP API 服务器（端口 `8082`），暴露 agent 列表、搜索、注册和卡片检索等端点。
- **灵活的发现范围** — 支持 `Cluster` 或 `Namespace` 级别的发现，配合标签选择器和命名空间过滤——在 API 层面和 Kubernetes API 层面均可生效。
- **注册策略** — 精细控制 `requireApproval`（需审批）、`requireHealthCheck`（需健康检查）和 `requireCardMatch`（需卡片匹配）。
- **全局健康检查默认值** — 通过 `A2ARegistry` CR 或 Dashboard 设置界面配置集群级别的健康检查默认间隔和超时时间。每个 Agent 可单独覆盖。
- **Dashboard 设置界面** — 在 Dashboard 中通过齿轮图标直接配置全局健康检查参数、注册策略和审批开关。
- **卡片匹配** — 可选校验拉取到的 agent card 是否与 CR 中声明的 spec 一致（名称、描述、版本、技能）。
- **审批流程** — 启用 `requireApproval` 时，新建 agent 保持 `Pending` 状态，直到运维人员手动添加 `Approved` 条件。
- **Agent 自动清理** — 连续 `Unreachable` 超过 7 天的 agent 会被自动删除。
- **认证支持** — 处于认证端点（bearer token、basic auth）之后的 agent，可通过引用 Kubernetes Secret 进行健康检查。
- **准入 Webhook** — 验证和变更 webhook 确保 `A2AAgent` 和 `A2ARegistry` 两种资源的 spec 合规，CRD schema 校验作为兜底。
- **多架构** — 预构建 Docker 镜像支持 `linux/amd64` 和 `linux/arm64`。
- **Prometheus 指标** — 端口 `8080` 暴露监控指标（agent 数量、健康检查延迟/失败次数、注册/注销计数、API 延迟）。
- **Kubernetes Events** — 关键状态变更（Ready、Unreachable、CardMismatch、AgentPruned）产生 Kubernetes Event 用于审计追溯。
- **Leader Election** — 可选的高可用 leader 选举。

---

## 架构

```
┌─────────────────────────────────────────────────────┐
│                  Kubernetes 集群                      │
│                                                      │
│  ┌──────────────────────────────────────────────┐   │
│  │           a2a-registry (操作器)                │   │
│  │                                               │   │
│  │  ┌─────────────┐  ┌──────────────┐           │   │
│  │  │  A2AAgent   │  │ A2ARegistry  │           │   │
│  │  │ Controller  │  │  Controller  │           │   │
│  │  └──────┬──────┘  └──────┬───────┘           │   │
│  │         │                │                    │   │
│  │         ▼                ▼                    │   │
│  │  ┌─────────────┐  ┌──────────────┐           │   │
│  │  │   健康检查   │  │   注册中心    │           │   │
│  │  │   Checker   │  │   API Server │           │   │
│  │  └──────┬──────┘  │   (:8082)    │           │   │
│  │         │         └──────┬───────┘           │   │
│  └─────────┼────────────────┼───────────────────┘   │
│            │                │                        │
│            ▼                ▼                        │
│  ┌──────────────────┐  ┌──────────────────┐         │
│  │   Agent Pod A    │  │   外部客户端       │        │
│  │  /.well-known/   │  │  POST /api/v1/    │        │
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

**核心组件：**

| 组件 | 描述 |
|---|---|
| **A2AAgent Controller** | 监听 `A2AAgent` CR，通过拉取 agent card 执行健康检查，执行注册策略（审批、卡片匹配、URL 唯一性），更新状态条件和阶段。 |
| **A2ARegistry Controller** | 监听 `A2ARegistry` CR，统计已注册/健康 agent 数量，将发现配置推送到 API 服务器，自动清理过期的不可达 agent。 |
| **Registry API Server** | HTTP 服务器，提供注册中心自身的 Agent Card、agent 发现/搜索端点，以及 HTTP 注册/注销功能。所有副本均可运行（无需 leader election）。 |
| **Agent Card Resolver** | 从 `/.well-known/agent-card.json` 拉取并解析 agent card（回退到 `/.well-known/agent.json`）。支持 bearer token 和 basic 认证。 |
| **Admission Webhooks** | 在创建/更新时验证并填充 `A2AAgent` 和 `A2ARegistry` 资源的默认值。 |
| **Metrics & Events** | 端口 `:8080` 提供 Prometheus 指标（agent 数量、健康检查延迟、注册/注销计数、API 延迟）。生命周期转换产生 Kubernetes Event。 |

---

## 快速开始

### 前置条件

- Go 1.25+
- 运行中的 Kubernetes 集群（v1.27+）
- 已配置好集群访问权限的 `kubectl`
- `kustomize`（用于部署）

### 1. 安装 CRD 并部署操作器

```bash
git clone https://github.com/terminus-io/a2a-registry.git
cd a2a-registry

# 方式 A：kubectl + kustomize
make install   # 安装 CRD
make deploy    # 部署操作器

# 方式 B：Helm
helm install a2a-registry deploy/helm/a2a-registry \
  --namespace a2a-registry-system --create-namespace
```

### 2. 创建注册中心

```bash
kubectl apply -f config/samples/a2a.io_v1_a2aregistry.yaml
```

### 3. 注册 Agent

```bash
# 方式一：通过 kubectl
kubectl apply -f config/samples/a2a.io_v1_a2aagent.yaml

# 方式二：通过 HTTP API
kubectl port-forward -n a2a-registry-system svc/a2a-registry-controller-manager 8082:8082
curl -X POST http://localhost:8082/api/v1/agents \
  -H "Content-Type: application/json" \
  -d '{
    "namespace": "outbound-agent",
    "name": "我的 Agent",
    "url": "http://my-agent.default.svc.cluster.local:9001",
    "skills": [{"id": "hello", "name": "打招呼"}],
    "tags": ["demo"]
  }'
```

### 4. 查询注册中心 API

```bash
# 列出所有 agent
curl http://localhost:8082/api/v1/agents

# 搜索 agent
curl "http://localhost:8082/api/v1/search?q=hello&tag=demo"

# 获取注册中心自身的 agent card
curl http://localhost:8082/.well-known/agent-card.json
```

### 5. Dashboard

访问 `http://localhost:8082/` 即可打开 Dashboard 界面，支持 Agent 列表、搜索过滤、注册、审批、详情查看、**全局设置**（健康检查间隔、注册策略、审批开关），以及中英文切换。

![Dashboard](docs/image/dashboard.png)

---

## 安装

### 从源码构建

```bash
make build          # 构建 bin/manager
make docker-build   # 构建 Docker 镜像
make docker-push    # 推送到镜像仓库
```

### 使用预构建镜像

```bash
export IMG=your-registry/a2a-registry:latest
make docker-build
make docker-push
make deploy
```

### 使用 Helm

```bash
git clone https://github.com/terminus-io/a2a-registry.git
cd a2a-registry

# 使用默认配置安装
helm install a2a-registry deploy/helm/a2a-registry \
  --namespace a2a-registry-system --create-namespace

# 使用自定义镜像安装
helm install a2a-registry deploy/helm/a2a-registry \
  --namespace a2a-registry-system --create-namespace \
  --set image.repository=your-registry/a2a-registry \
  --set image.tag=latest
```

### 多架构构建

```bash
make docker-buildx PLATFORMS=linux/amd64,linux/arm64
```

### 卸载

```bash
make undeploy       # 移除操作器部署
make uninstall      # 移除 CRD
```

---

## 使用指南

### 创建注册中心

`A2ARegistry` 是集群级别的资源，定义 agent 发现、注册策略和 API 服务器的全局配置。

```yaml
apiVersion: a2a.io/v1
kind: A2ARegistry
metadata:
  name: default-registry
spec:
  discovery:
    scope: "Cluster"           # "Cluster" 或 "Namespace"
    labelSelector: "app=my-agent"
    # namespaces: ["ns1", "ns2"]
  registration:
    requireApproval: false     # 新 agent 是否需要手动审批
    requireHealthCheck: true   # 是否需要 URL 可达才能标记 Ready
    requireCardMatch: false    # 是否校验拉取的 card 与 spec 一致
  healthCheck:
    intervalSeconds: 120       # 集群级默认健康检查间隔
    timeoutSeconds: 15
  apiServer:
    port: 8082
    bindAddress: "0.0.0.0"
    # tlsCertRef:              # 可选 TLS 证书
    #   name: registry-tls
```

**注册策略说明：**

| 字段 | 默认值 | 说明 |
|---|---|---|
| `requireApproval` | `false` | 为 true 时，新 agent 保持 `Pending`，直到运维人员手动给其 status 添加 `Approved` 条件。 |
| `requireHealthCheck` | `true` | 为 false 时，agent 跳过健康检查直接标记为 `Ready`。 |
| `requireCardMatch` | `false` | 为 true 时，拉取到的 agent card 必须与 CR spec 一致（名称、描述、版本、技能）。不一致则进入 `Error`。 |

**健康检查默认值（集群级别）：**

| 字段 | 默认值 | 说明 |
|---|---|---|
| `spec.healthCheck.intervalSeconds` | `60` | 默认健康检查间隔。可通过 CR 或 Dashboard 设置界面配置。最小值：`1`。 |
| `spec.healthCheck.timeoutSeconds` | `10` | 默认健康检查超时时间。不能超过间隔。 |

> **优先级：** 每个 Agent 的 `spec.healthCheck` > 全局 `A2ARegistry.spec.healthCheck` > 硬编码默认值（60秒）。
>
> 全局默认值也可通过 **Dashboard 设置界面**（齿轮图标）进行配置，它会通过 `PUT /api/v1/config` 更新 `A2ARegistry` CR。

### 注册 Agent

`A2AAgent` 是命名空间级别的资源，代表集群中的单个 A2A agent。

```yaml
apiVersion: a2a.io/v1
kind: A2AAgent
metadata:
  name: hello-world-agent
  namespace: outbound-agent
spec:
  name: "Hello World Agent"
  description: "一个返回问候语的简单示例 agent"
  version: "1.0.0"
  url: "http://agent-service.default.svc.cluster.local:9001"
  capabilities:
    streaming: true
    pushNotifications: false
  skills:
    - id: "hello_world"
      name: "你好，世界！"
      description: "返回一条问候信息"
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
  authentication:
    schemes:
      - "bearer"
    secretRef:
      name: "my-agent-credentials"
  healthCheck:
    intervalSeconds: 60
    timeoutSeconds: 10
    failureThreshold: 3
```

**关键字段：**

| 字段 | 说明 |
|---|---|
| `spec.url` | Agent 的基础端点 URL（必须为 `http://` 或 `https://`） |
| `spec.skills` | Agent 可执行的技能列表，每个技能需唯一 `id` |
| `spec.enabled` | 设为 `false` 可禁用健康检查并从发现结果中移除 |
| `spec.capabilities` | 声明是否支持 `streaming` 和/或 `pushNotifications` |
| `spec.authentication` | 可选认证配置（`schemes` + `secretRef`），用于健康检查受保护的 agent 端点 |
| `spec.healthCheck` | 每个 agent 的健康检查参数覆写（默认：interval=60s, timeout=10s, failureThreshold=3）。若不设置，回退到全局 `A2ARegistry.spec.healthCheck`。 |
| `spec.healthCheck.intervalSeconds` | 覆盖全局默认健康检查间隔。优先级：per-agent > 全局 registry > 60秒。 |
| `spec.healthCheck.failureThreshold` | 连续多少次失败后 agent 变为 `Unreachable`（默认 3） |
| `spec.tags` | 用于搜索和过滤的任意标签 |

**状态字段：**

| 字段 | 说明 |
|---|---|
| `status.phase` | 生命周期阶段：`Pending`、`Ready`、`Error`、`Unreachable` |
| `status.health` | 健康状态：`Healthy`、`Unhealthy`、`Unknown` |
| `status.lastHeartbeat` | 最后一次成功健康检查的时间戳 |
| `status.agentCardHash` | 拉取到的 agent card 的 SHA256 哈希值 |
| `status.consecutiveFailures` | 连续健康检查失败次数 |
| `status.registeredAt` | Agent 首次注册的时间戳 |
| `status.conditions` | Kubernetes 标准条件（`HealthChecked`、`Ready`、`Approved`） |

### 通过 API 发现 Agent

注册中心 API 服务器暴露以下端点：

#### 列出 Agent

```bash
GET /api/v1/agents
GET /api/v1/agents?namespace=default
GET /api/v1/agents?tags=demo,example
GET /api/v1/agents?skill=hello_world
```

#### 注册 Agent

> `namespace` 为选填字段。不填或为空时，Agent 将创建在 **`outbound-agent`** 命名空间中（Operator 启动时自动创建）。也可自行指定其他命名空间。

```bash
POST /api/v1/agents
Content-Type: application/json

{
  "name": "我的 Agent",
  "namespace": "outbound-agent",
  "url": "http://agent.default.svc.cluster.local:9001",
  "description": "一个示例 agent",
  "version": "1.0.0",
  "skills": [
    {"id": "greeting", "name": "打招呼"}
  ],
  "tags": ["demo"],
  "streaming": false,
  "pushNotifications": false,
  "protocolVersion": "1.0"
}
```

#### 获取单个 Agent

```bash
GET /api/v1/agents/{name}
GET /api/v1/agents/{name}/card    # 返回 A2A Agent Card
```

#### 注销 Agent

```bash
DELETE /api/v1/agents/{name}?namespace=default
```

#### 搜索 Agent

```bash
GET /api/v1/search?q=hello                  # 全文搜索
GET /api/v1/search?tag=demo                 # 按标签过滤
GET /api/v1/search?skill=hello_world        # 按技能过滤
GET /api/v1/search?capability=streaming     # 按能力过滤
```

#### 注册中心自身的 Agent Card

```bash
GET /.well-known/agent-card.json
GET /.well-known/agent.json
```

注册中心将自己作为 agent 暴露，拥有 "Agent Discovery" 和 "Agent Search" 两个技能。

### Agent 审批流程

当注册中心配置中启用了 `registration.requireApproval`：

1. 新 agent 创建后处于 `Pending` 阶段，状态信息为 `"等待审批。"`
2. 运维人员通过 patch 其 status 来审批：

```bash
kubectl patch a2aa my-agent --type=merge --subresource=status -p \
  '{"status":{"conditions":[{"type":"Approved","status":"True","reason":"Approved","message":"Agent 已审批通过。"}]}}'
```

3. 下一次调和检测到 `Approved` 条件后，继续执行健康检查流程。

### 受认证保护的 Agent Card

如果 agent 的 `/.well-known/agent-card.json` 需要认证：

```yaml
spec:
  authentication:
    schemes:
      - "bearer"              # 或 "basic"
    secretRef:
      name: "agent-auth"      # 同命名空间下的 Kubernetes Secret
```

**Bearer token 类型的 Secret：**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: agent-auth
type: Opaque
stringData:
  token: "my-bearer-token"
```

**Basic auth 类型的 Secret：**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: agent-auth
type: Opaque
stringData:
  username: "myuser"
  password: "mypass"
```

---

## API 参考

### 自定义资源定义

| CRD | 作用域 | 简写 | 描述 |
|---|---|---|---|
| `A2AAgent` | Namespaced | `a2aa` | 表示一个已注册的 A2A agent |
| `A2ARegistry` | Cluster | `a2areg` | 全局注册中心配置 |

### kubectl 快捷命令

```bash
kubectl get a2aa                    # 列出所有 A2AAgent
kubectl get a2areg                  # 列出所有 A2ARegistry
kubectl describe a2aa <name>        # 查看 agent 详情
kubectl get a2aa -o wide            # 显示额外列（Phase、Health、URL）
```

### Agent 生命周期

```
  [创建] ──► Pending ──► Ready ◄──── 健康检查通过
                │            │
                ▼            ▼
              Error    Unreachable
                │            │
                └─────┬──────┘
                      ▼
                    Ready（恢复后）
```

- **Pending**：初始状态，或等待审批，或手动禁用。
- **Ready**：健康检查通过；agent 可被发现。
- **Error**：健康检查失败（连续失败次数未达到 `failureThreshold`）。
- **Unreachable**：连续失败次数达到 `failureThreshold`。在此状态下持续 7 天后会被自动清理。

### HTTP API 端点

| 方法 | 路径 | 描述 |
|---|---|---|
| `GET` | `/api/v1/agents` | 列出所有 agent（支持过滤：`?namespace=`、`?tags=`、`?skill=`） |
| `POST` | `/api/v1/agents` | 注册新 agent（JSON 请求体） |
| `GET` | `/api/v1/agents/{name}` | 获取单个 agent |
| `DELETE` | `/api/v1/agents/{name}` | 注销 agent |
| `GET` | `/api/v1/agents/{name}/card` | 获取 agent 的 A2A Agent Card |
| `GET` | `/api/v1/search` | 搜索 agent（`?q=`、`?tag=`、`?skill=`、`?capability=`） |
| `GET` | `/api/v1/config` | 获取注册中心配置（策略、健康检查默认值） |
| `PUT` | `/api/v1/config` | 更新注册中心配置（Dashboard 设置界面） |
| `GET` | `/.well-known/agent-card.json` | 注册中心自身的 Agent Card |
| `GET` | `/.well-known/agent.json` | 注册中心 Agent Card（兼容旧版路径） |

---

## 配置

### 操作器启动参数

| 参数 | 默认值 | 描述 |
|---|---|---|
| `--metrics-bind-address` | `:8080` | Prometheus 指标端点 |
| `--health-probe-bind-address` | `:8081` | 健康/就绪探针端点 |
| `--registry-api-bind-address` | `:8082` | 注册中心发现 API |
| `--leader-elect` | `false` | 启用 leader election |
| `--leader-election-id` | `a2a-registry` | Leader election 标识 |

### 环境变量

| 变量 | 默认值 | 描述 |
|---|---|---|
| `ENABLE_WEBHOOKS` | (启用) | 设置为 `"false"` 禁用准入 webhook |

---

## 监控

### Prometheus 指标

所有指标以 `a2a_registry_` 为前缀，通过端口 `:8080` 暴露。

| 指标 | 类型 | 标签 | 描述 |
|---|---|---|---|
| `a2a_registry_agent_count` | GaugeVec | `phase`, `health` | 按阶段和健康状态统计的 agent 数量 |
| `a2a_registry_health_check_duration_seconds` | Histogram | — | Agent 健康检查延迟 |
| `a2a_registry_health_check_failures_total` | Counter | — | 健康检查失败总数 |
| `a2a_registry_agent_card_fetch_errors_total` | Counter | — | Agent card 拉取错误总数 |
| `a2a_registry_registrations_total` | Counter | — | 注册总数 |
| `a2a_registry_deregistrations_total` | Counter | — | 注销总数 |
| `a2a_registry_api_request_duration_seconds` | HistogramVec | `endpoint`, `method` | 注册中心 API 请求延迟 |

### Kubernetes Events

状态转换会发出 Kubernetes Event，可通过 `kubectl describe a2aa <name>` 查看：

| 事件 | 类型 | 含义 |
|---|---|---|
| `FinalizerRemoved` | Normal | Agent 删除完成 |
| `HealthCheckRecovered` | Normal | Agent 从 Error/Unreachable 恢复到 Ready |
| `CardMismatch` | Warning | 拉取到的 card 与 spec 不匹配 |
| `HealthCheckFailed` | Warning | 单次健康检查失败 |
| `AgentUnreachable` | Warning | Agent 超过失败阈值被标记为 Unreachable |
| `AgentPruned` | Normal | 过期 agent 被自动清理（超过 7 天） |

---

## 开发

```bash
make build          # 构建二进制文件
make run            # 本地运行（不在集群内）
make test           # 运行测试（需要 envtest）
make generate manifests  # 生成 CRD 清单和 DeepCopy 方法
make fmt            # 格式化代码
make vet            # 静态检查

# 构建并推送 Docker 镜像
export IMG=your-registry/a2a-registry:latest
make docker-build
make docker-push
```

### 项目结构

```
.
├── api/v1/                    # CRD 类型定义 + webhook
├── cmd/manager/               # 操作器入口
├── config/
│   ├── crd/bases/             # 生成的 CRD YAML 清单
│   ├── rbac/                  # RBAC 角色和绑定
│   ├── manager/               # Deployment 清单
│   ├── default/               # 顶层 kustomization
│   └── samples/               # 示例 CR
├── controllers/               # 调和逻辑
├── internal/
│   ├── healthcheck/           # Agent 健康检查
│   ├── metrics/               # Prometheus 指标定义
│   └── registry/              # API 服务器 + handler + agent card 解析器
├── examples/hello-agent/      # 示例 A2A agent
├── deploy/helm/a2a-registry/  # Helm Chart
├── vendor/                    # Vendor 依赖
├── Dockerfile                 # 多阶段容器构建
└── Makefile                   # 构建和部署目标
```

---

## 示例

[`examples/hello-agent/`](examples/hello-agent/) 目录包含一个功能完整的 A2A agent 实现：

- 在 `/.well-known/agent-card.json` 提供服务卡片
- 在 `/invoke` 接受 JSON-RPC 调用
- 包含完整的 `deploy.yaml`（Namespace、Deployment、Service 和 A2AAgent CR）

```bash
cd examples/hello-agent
kubectl apply -f deploy.yaml
```

---

## 相关项目

- [A2A 协议规范](https://github.com/google/A2A) — Agent 间通信的开放标准
- [a2a-go](https://github.com/a2aproject/a2a-go) — A2A 协议的 Go SDK
- [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) — 构建 Kubernetes 操作器的框架

---

## 许可证

本项目基于 Apache License 2.0 许可。详见 [LICENSE](LICENSE)。
