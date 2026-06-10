# A2A Registry v0.1.0

首个公开发布版本。

## Features

### Agent 管理
- **Kubernetes-native** — Agent 以 CRD（`A2AAgent`、`A2ARegistry`）形式定义，Kubernetes 即数据源
- **HTTP 注册 API** — Agent 可通过 `POST /api/v1/agents` 自行注册，`DELETE /api/v1/agents/{name}` 注销
- **自动健康检查** — 定期拉取每个 Agent 的 `/.well-known/agent-card.json`，追踪生命周期：`Pending → Ready → Error → Unreachable`
- **可配置失败阈值** — 连续 N 次失败后标记为 `Unreachable`，7 天后自动清理
- **URL 冲突检测** — 自动检测并拒绝重复 URL

### 注册策略
- `requireApproval` — 新 Agent 需手动审批后才能进行健康检查
- `requireHealthCheck` — URL 可达才标记 Ready
- `requireCardMatch` — 校验拉取的 Agent Card 与 CR spec 一致性
- Dashboard 一键审批

### 全局配置
- **健康检查默认值**：`A2ARegistry` CR 中设置集群级别的默认间隔和超时，per-agent 可覆盖
- **默认命名空间 `outbound-agent`**：Operator 启动时自动创建，API 注册时 namespace 可选，不填默认使用此命名空间
- **Dashboard 设置界面**：齿轮图标打开，预设间隔（1s/15s/30s/1min/自定义）、审批/健康检查开关

### Dashboard
- Agent 列表、搜索、过滤、注册、审批、删除
- 详情查看（Skills、Conditions、元数据）
- 设置界面（健康检查参数、注册策略）
- 中 / 英 / 日 三语切换

### 认证
- Bearer token / Basic auth 支持，通过 Kubernetes Secret 引用凭据

### 可观测性
- **Prometheus Metrics**（`:8080`）：Agent 数量、健康检查延迟/失败、注册/注销计数、API 延迟
- **Kubernetes Events**：Ready、Unreachable、CardMismatch、AgentPruned 等状态变更事件

### 其他
- 多架构 Docker 镜像（`linux/amd64`、`linux/arm64`）
- 准入 Webhook（验证 + 默认值注入）
- 灵活的发现范围（Cluster / Namespace + Label Selector）
- 可选 Leader Election（HA 部署）

## Bug Fixes

- 修复新 Agent 首次 Reconcile 后僵死的问题：`GenerationChangedPredicate` 过滤了 finalizer 更新事件，导致 Agent 添加 finalizer 后永远不再被健康检查

## Breaking Changes

- 默认命名空间从 `default` 改为 `outbound-agent`
- API 注册接口 namespace 从 URL query 参数改为请求 body 中的 `"namespace"` 字段

## API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/agents` | 列出 Agent（`?namespace=` / `?tags=` / `?skill=`） |
| `POST` | `/api/v1/agents` | 注册 Agent（JSON body 含可选 `namespace`） |
| `GET` | `/api/v1/agents/{name}` | 获取 Agent |
| `DELETE` | `/api/v1/agents/{name}` | 注销 Agent |
| `GET` | `/api/v1/agents/{name}/card` | 获取 Agent 的 A2A Agent Card |
| `GET` | `/api/v1/agents/{name}/approve` | Dashboard 审批 Agent |
| `GET` | `/api/v1/search` | 搜索（`?q=` / `?tag=` / `?skill=` / `?capability=`） |
| `GET` | `/api/v1/config` | 获取注册中心全局配置 |
| `PUT` | `/api/v1/config` | 更新注册中心全局配置 |
| `GET` | `/.well-known/agent-card.json` | 注册中心自身 Agent Card |

## 安装

```bash
git clone https://github.com/terminus-io/a2a-registry.git
cd a2a-registry
make install   # 安装 CRD
make deploy    # 部署 Operator
```

## 快速开始

```bash
# 注册 Agent
curl -X POST http://localhost:8082/api/v1/agents \
  -H "Content-Type: application/json" \
  -d '{
    "namespace": "outbound-agent",
    "name": "My Agent",
    "url": "http://my-agent:9001",
    "skills": [{"id": "hello", "name": "Hello"}],
    "tags": ["demo"]
  }'

# 打开 Dashboard
open http://localhost:8082/
```
