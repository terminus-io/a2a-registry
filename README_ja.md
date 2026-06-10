# A2A Registry
日本語版 | [English Version](README.md) | [中文版](README_zh.md)

[![Go Version](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-v1.27+-326CE5?style=flat&logo=kubernetes)](https://kubernetes.io/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

**Kubernetes ネイティブな Agent レジストリ** — [Agent-to-Agent (A2A) プロトコル](https://github.com/google/A2A)（Google が提唱するエージェント間直接通信のオープン標準）に対応。

[kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) フレームワークで構築され、カスタムリソース定義（CRD）によりクラスタ内でエージェントの登録、検出、ヘルスチェック、検索を管理します。

---

## 目次

- [特徴](#特徴)
- [アーキテクチャ](#アーキテクチャ)
- [クイックスタート](#クイックスタート)
- [インストール](#インストール)
- [使用方法](#使用方法)
  - [レジストリの作成](#レジストリの作成)
  - [エージェントの登録](#エージェントの登録)
  - [API 経由のエージェント検出](#api-経由のエージェント検出)
  - [エージェント承認ワークフロー](#エージェント承認ワークフロー)
  - [認証付きエージェントカード](#認証付きエージェントカード)
- [API リファレンス](#api-リファレンス)
- [設定](#設定)
- [監視](#監視)
- [開発](#開発)
- [サンプル](#サンプル)
- [ライセンス](#ライセンス)

---

## 特徴

- **Kubernetes ネイティブ** — エージェントは CRD（`A2AAgent`、`A2ARegistry`）として定義されます。外部データベースは不要で、Kubernetes が信頼できる唯一の情報源（Source of Truth）です。
- **HTTP 登録 API** — エージェントは `POST /api/v1/agents` で自己登録、`DELETE /api/v1/agents/{name}` で登録解除が可能です。`kubectl apply` にも対応しています。
- **自動ヘルスチェック** — 各エージェントの `/.well-known/agent-card.json` から定期的に [Agent Card](https://github.com/google/A2A/blob/main/specification/a2a.json) を取得し、ライフサイクルフェーズ（`Pending → Ready → Error → Unreachable`）を追跡します。
- **設定可能な障害閾値** — エージェントごとに `failureThreshold` を設定し、連続何回の失敗で `Unreachable` とマークするかを制御できます。
- **URL 重複検出** — 重複するエージェント URL を検出し、競合するエージェントを `Error` にマークします。
- **組み込み検出 API** — オペレーター内部で RESTful HTTP API サーバー（ポート `8082`）が動作し、エージェント一覧、検索、登録、カード取得のエンドポイントを提供します。
- **柔軟な検出スコープ** — `Cluster` または `Namespace` レベルで検出範囲を設定し、ラベルセレクターと名前空間フィルターを API レベルと Kubernetes API レベルの両方で適用できます。
- **登録ポリシー** — `requireApproval`（承認要）、`requireHealthCheck`（ヘルスチェック要）、`requireCardMatch`（カード一致要）をレジストリごとに細かく制御できます。
- **グローバルヘルスチェックデフォルト** — `A2ARegistry` CR または Dashboard 設定画面から、クラスタ全体のデフォルトヘルスチェック間隔とタイムアウトを設定できます。エージェントごとの上書きが優先されます。
- **Dashboard 設定 UI** — 歯車アイコンから Dashboard 上で直接グローバルヘルスチェックパラメータ、登録ポリシー、承認トグルを設定できます。
- **カードマッチング** — 取得したエージェントカードが CR の spec と一致するか（名前、説明、バージョン、スキル）をオプションで検証します。
- **承認ワークフロー** — `requireApproval` が有効な場合、新しいエージェントはオペレーターが `Approved` 条件を追加するまで `Pending` 状態を維持します。
- **エージェント自動クリーンアップ** — 連続 7 日間 `Unreachable` 状態が続いたエージェントは自動的に削除されます。
- **認証サポート** — 認証エンドポイント（bearer token、basic auth）の背後にあるエージェントに対し、Kubernetes Secret を参照してヘルスチェックを実行できます。
- **アドミッション Webhook** — 検証および変更用の Webhook が `A2AAgent` と `A2ARegistry` の両方のリソースに対し spec の正当性を強制します。CRD スキーマ検証がセーフティネットとして機能します。
- **マルチアーキテクチャ** — ビルド済み Docker イメージが `linux/amd64` と `linux/arm64` に対応しています。
- **Prometheus メトリクス** — ポート `8080` で監視用メトリクスを公開（エージェント数、ヘルスチェック遅延/失敗数、登録/解除数、API 遅延）。
- **Kubernetes Events** — 主要な状態遷移（Ready、Unreachable、CardMismatch、AgentPruned）が Kubernetes Event として発行され、監査証跡として利用できます。
- **Leader Election** — 高可用性デプロイメント向けのオプションのリーダー選出。

---

## アーキテクチャ

```
┌─────────────────────────────────────────────────────┐
│                  Kubernetes クラスタ                   │
│                                                      │
│  ┌──────────────────────────────────────────────┐   │
│  │           a2a-registry (オペレーター)           │   │
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
│  │   Agent Pod A    │  │  外部クライアント  │        │
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

**主要コンポーネント：**

| コンポーネント | 説明 |
|---|---|
| **A2AAgent Controller** | `A2AAgent` CR を監視し、エージェントカードを取得してヘルスチェックを実行、登録ポリシー（承認、カード一致、URL 一意性）を適用し、ステータス条件とフェーズを更新します。 |
| **A2ARegistry Controller** | `A2ARegistry` CR を監視し、登録済み/健全なエージェント数をカウント、検出設定を API サーバーにプッシュ、古い到達不能エージェントを自動クリーンアップします。 |
| **Registry API Server** | HTTP サーバー。レジストリ自身の Agent Card の提供、エージェント検出/検索エンドポイント、HTTP 経由のエージェント登録/解除を処理します。全レプリカで実行可能（リーダー選出不要）。 |
| **Agent Card Resolver** | `/.well-known/agent-card.json` からエージェントカードを取得・解析します（`/.well-known/agent.json` へのフォールバックあり）。Bearer token と Basic 認証をサポート。 |
| **Admission Webhooks** | 作成/更新時に `A2AAgent` と `A2ARegistry` の両方のリソースを検証し、デフォルト値を設定します。 |
| **Metrics & Events** | ポート `:8080` で Prometheus メトリクス（エージェント数、ヘルスチェック遅延、登録/解除数、API 遅延）を提供。ライフサイクル遷移時に Kubernetes Event を発行。 |

---

## クイックスタート

### 前提条件

- Go 1.25+
- 稼働中の Kubernetes クラスタ（v1.27+）
- クラスタアクセスが設定済みの `kubectl`
- `kustomize`（デプロイ用）

### 1. CRD のインストールとオペレーターのデプロイ

```bash
git clone https://github.com/terminus-io/a2a-registry.git
cd a2a-registry

# 方法 A：kubectl + kustomize
make install   # CRD のインストール
make deploy    # オペレーターのデプロイ

# 方法 B：Helm
helm install a2a-registry deploy/helm/a2a-registry \
  --namespace a2a-registry-system --create-namespace
```

### 2. レジストリの作成

```bash
kubectl apply -f config/samples/a2a.io_v1_a2aregistry.yaml
```

### 3. エージェントの登録

```bash
# kubectl 経由
kubectl apply -f config/samples/a2a.io_v1_a2aagent.yaml

# または HTTP API 経由
kubectl port-forward -n a2a-registry-system svc/a2a-registry-controller-manager 8082:8082
curl -X POST http://localhost:8082/api/v1/agents \
  -H "Content-Type: application/json" \
  -d '{
    "namespace": "outbound-agent",
    "name": "My Agent",
    "url": "http://my-agent.default.svc.cluster.local:9001",
    "skills": [{"id": "hello", "name": "Hello"}],
    "tags": ["demo"]
  }'
```

### 4. レジストリ API のクエリ

```bash
# 全エージェントの一覧表示
curl http://localhost:8082/api/v1/agents

# エージェントの検索
curl "http://localhost:8082/api/v1/search?q=hello&tag=demo"

# レジストリ自身の Agent Card の取得
curl http://localhost:8082/.well-known/agent-card.json
```

### 5. Dashboard

`http://localhost:8082/` にアクセスすると Dashboard が開きます。エージェント一覧、検索フィルター、登録、承認、詳細表示、**グローバル設定**（ヘルスチェック間隔、登録ポリシー、承認トグル）、および多言語対応（日本語 / 中国語 / 英語）をサポートしています。

![Dashboard](docs/image/dashboard.png)

---

## インストール

### ソースからのビルド

```bash
make build          # bin/manager をビルド
make docker-build   # Docker イメージをビルド
make docker-push    # レジストリにプッシュ
```

### ビルド済みイメージの使用

```bash
export IMG=your-registry/a2a-registry:latest
make docker-build
make docker-push
make deploy
```

### Helm の使用

```bash
git clone https://github.com/terminus-io/a2a-registry.git
cd a2a-registry

# デフォルト設定でインストール
helm install a2a-registry deploy/helm/a2a-registry \
  --namespace a2a-registry-system --create-namespace

# カスタムイメージでインストール
helm install a2a-registry deploy/helm/a2a-registry \
  --namespace a2a-registry-system --create-namespace \
  --set image.repository=your-registry/a2a-registry \
  --set image.tag=latest
```

### マルチアーキテクチャビルド

```bash
make docker-buildx PLATFORMS=linux/amd64,linux/arm64
```

### アンインストール

```bash
make undeploy       # オペレーターデプロイメントの削除
make uninstall      # CRD の削除
```

---

## 使用方法

### レジストリの作成

`A2ARegistry` リソースはクラスタスコープで、エージェント検出、登録ポリシー、API サーバーのグローバル設定を定義します。

```yaml
apiVersion: a2a.io/v1
kind: A2ARegistry
metadata:
  name: default-registry
spec:
  discovery:
    scope: "Cluster"           # "Cluster" または "Namespace"
    labelSelector: "app=my-agent"
    # namespaces: ["ns1", "ns2"]
  registration:
    requireApproval: false     # 新規エージェントに手動承認を要求するか
    requireHealthCheck: true   # Ready とマークする前に URL 到達可能を要求するか
    requireCardMatch: false    # 取得したカードが spec と一致するか検証するか
  healthCheck:
    intervalSeconds: 120       # クラスタ全体のデフォルトヘルスチェック間隔
    timeoutSeconds: 15
  apiServer:
    port: 8082
    bindAddress: "0.0.0.0"
    # tlsCertRef:              # オプションの TLS 証明書
    #   name: registry-tls
```

**登録ポリシー：**

| フィールド | デフォルト | 説明 |
|---|---|---|
| `requireApproval` | `false` | true の場合、新しいエージェントはオペレーターが status に `Approved` 条件を追加するまで `Pending` のままです。 |
| `requireHealthCheck` | `true` | false の場合、エージェントはヘルスチェックなしで即座に `Ready` とマークされます。 |
| `requireCardMatch` | `false` | true の場合、取得したエージェントカードが CR の spec（名前、説明、バージョン、スキル）と一致する必要があります。不一致の場合 `Error` フェーズになります。 |

**ヘルスチェックデフォルト（クラスタ全体）：**

| フィールド | デフォルト | 説明 |
|---|---|---|
| `spec.healthCheck.intervalSeconds` | `60` | デフォルトのヘルスチェック間隔。CR または Dashboard 設定画面で設定可能。最小値: `1`。 |
| `spec.healthCheck.timeoutSeconds` | `10` | デフォルトのヘルスチェックタイムアウト。間隔を超えてはなりません。 |

> **優先順位:** エージェントごとの `spec.healthCheck` > グローバル `A2ARegistry.spec.healthCheck` > ハードコードフォールバック（60秒）。
>
> グローバルデフォルトは **Dashboard 設定 UI**（歯車アイコン）からも設定でき、`PUT /api/v1/config` 経由で `A2ARegistry` CR を更新します。

### エージェントの登録

`A2AAgent` リソースはクラスタ内の単一の A2A エージェントを表します。名前空間スコープです。

```yaml
apiVersion: a2a.io/v1
kind: A2AAgent
metadata:
  name: hello-world-agent
  namespace: outbound-agent
spec:
  name: "Hello World Agent"
  description: "挨拶を返すシンプルなサンプルエージェント"
  version: "1.0.0"
  url: "http://agent-service.default.svc.cluster.local:9001"
  capabilities:
    streaming: true
    pushNotifications: false
  skills:
    - id: "hello_world"
      name: "Hello, world!"
      description: "挨拶メッセージを返します"
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

**主要フィールド：**

| フィールド | 説明 |
|---|---|
| `spec.url` | エージェントのベースエンドポイント URL（`http://` または `https://` 必須） |
| `spec.skills` | エージェントが実行できるスキルのリスト。各スキルには一意の `id` が必要 |
| `spec.enabled` | `false` にするとヘルスチェックを無効化し、検出結果から除外 |
| `spec.capabilities` | `streaming` や `pushNotifications` のサポートを宣言 |
| `spec.authentication` | オプションの認証設定（`schemes` + `secretRef`）。保護されたエージェントエンドポイントのヘルスチェック用 |
| `spec.healthCheck` | エージェントごとのヘルスチェック上書き（デフォルト: interval=60s, timeout=10s, failureThreshold=3）。未設定の場合はグローバル `A2ARegistry.spec.healthCheck` にフォールバック |
| `spec.healthCheck.intervalSeconds` | グローバルデフォルトのヘルスチェック間隔を上書き。優先順位: per-agent > グローバルレジストリ > 60秒 |
| `spec.healthCheck.failureThreshold` | エージェントが `Unreachable` になるまでの連続失敗回数（デフォルト: 3） |
| `spec.tags` | 検索とフィルタリング用の任意のタグ |

**ステータスフィールド：**

| フィールド | 説明 |
|---|---|
| `status.phase` | ライフサイクルフェーズ: `Pending`、`Ready`、`Error`、`Unreachable` |
| `status.health` | ヘルス状態: `Healthy`、`Unhealthy`、`Unknown` |
| `status.lastHeartbeat` | 最後に成功したヘルスチェックのタイムスタンプ |
| `status.agentCardHash` | 取得したエージェントカードの SHA256 ハッシュ |
| `status.consecutiveFailures` | 連続ヘルスチェック失敗回数 |
| `status.registeredAt` | エージェントが最初に登録されたタイムスタンプ |
| `status.conditions` | Kubernetes 標準条件（`HealthChecked`、`Ready`、`Approved`） |

### API 経由のエージェント検出

レジストリ API サーバーは以下のエンドポイントを公開します：

#### エージェント一覧

```bash
GET /api/v1/agents
GET /api/v1/agents?namespace=default
GET /api/v1/agents?tags=demo,example
GET /api/v1/agents?skill=hello_world
```

#### エージェントの登録

> `namespace` は任意項目です。省略または空の場合、エージェントは **`outbound-agent`** 名前空間に作成されます（オペレーター起動時に自動生成）。独自の名前空間を指定することも可能です。

```bash
POST /api/v1/agents
Content-Type: application/json

{
  "name": "My Agent",
  "namespace": "outbound-agent",
  "url": "http://agent.default.svc.cluster.local:9001",
  "description": "サンプルエージェント",
  "version": "1.0.0",
  "skills": [
    {"id": "greeting", "name": "Greeting"}
  ],
  "tags": ["demo"],
  "streaming": false,
  "pushNotifications": false,
  "protocolVersion": "1.0"
}
```

#### 特定のエージェントの取得

```bash
GET /api/v1/agents/{name}
GET /api/v1/agents/{name}/card    # A2A Agent Card を返す
```

#### エージェントの登録解除

```bash
DELETE /api/v1/agents/{name}?namespace=default
```

#### エージェントの検索

```bash
GET /api/v1/search?q=hello                  # 全文検索
GET /api/v1/search?tag=demo                 # タグでフィルタ
GET /api/v1/search?skill=hello_world        # スキルでフィルタ
GET /api/v1/search?capability=streaming     # ケイパビリティでフィルタ
```

#### レジストリ自身の Agent Card

```bash
GET /.well-known/agent-card.json
GET /.well-known/agent.json
```

レジストリは自身をエージェントとして公開し、「Agent Discovery」と「Agent Search」のスキルを持ちます。

### エージェント承認ワークフロー

レジストリ設定で `registration.requireApproval` が有効な場合：

1. 新しいエージェントは `Pending` フェーズで作成され、ステータスに「承認待ち」と表示されます。
2. オペレーターがエージェントの status をパッチして承認します：

```bash
kubectl patch a2aa my-agent --type=merge --subresource=status -p \
  '{"status":{"conditions":[{"type":"Approved","status":"True","reason":"Approved","message":"Agent approved."}]}}'
```

3. 次回の調整で `Approved` 条件を検出し、ヘルスチェックフローに進みます。

> Dashboard の「承認」ボタンからもエージェントを承認できます。

### 認証付きエージェントカード

エージェントの `/.well-known/agent-card.json` が認証を要求する場合：

```yaml
spec:
  authentication:
    schemes:
      - "bearer"              # または "basic"
    secretRef:
      name: "agent-auth"      # 同じ名前空間の Kubernetes Secret
```

**Bearer token 用 Secret：**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: agent-auth
type: Opaque
stringData:
  token: "my-bearer-token"
```

**Basic 認証用 Secret：**

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

## API リファレンス

### カスタムリソース定義

| CRD | スコープ | 短縮名 | 説明 |
|---|---|---|---|
| `A2AAgent` | Namespaced | `a2aa` | 登録済みの A2A エージェントを表す |
| `A2ARegistry` | Cluster | `a2areg` | グローバルレジストリ設定 |

### kubectl ショートカット

```bash
kubectl get a2aa                    # 全 A2AAgent の一覧表示
kubectl get a2areg                  # 全 A2ARegistry の一覧表示
kubectl describe a2aa <name>        # エージェントの詳細表示
kubectl get a2aa -o wide            # 追加カラムの表示（Phase, Health, URL）
```

### エージェントのライフサイクル

```
  [作成] ──► Pending ──► Ready ◄──── ヘルスチェック成功
                │            │
                ▼            ▼
              Error    Unreachable
                │            │
                └─────┬──────┘
                      ▼
                    Ready（復旧後）
```

- **Pending**: 初期状態、承認待ち、または手動で無効化された状態。
- **Ready**: ヘルスチェック成功。エージェントは検出可能。
- **Error**: ヘルスチェック失敗（連続失敗回数が `failureThreshold` 未満）。
- **Unreachable**: `failureThreshold` 回連続でヘルスチェックに失敗。この状態が 7 日間続くと自動的にクリーンアップされます。

### HTTP API エンドポイント

| メソッド | パス | 説明 |
|---|---|---|
| `GET` | `/api/v1/agents` | 全エージェントの一覧表示（フィルタ: `?namespace=`, `?tags=`, `?skill=`） |
| `POST` | `/api/v1/agents` | 新規エージェントの登録（JSON ボディ） |
| `GET` | `/api/v1/agents/{name}` | 単一エージェントの取得 |
| `DELETE` | `/api/v1/agents/{name}` | エージェントの登録解除 |
| `GET` | `/api/v1/agents/{name}/card` | エージェントの A2A Agent Card の取得 |
| `GET` | `/api/v1/search` | エージェントの検索（`?q=`, `?tag=`, `?skill=`, `?capability=`） |
| `GET` | `/api/v1/config` | レジストリ設定の取得（ポリシー、ヘルスチェックデフォルト） |
| `PUT` | `/api/v1/config` | レジストリ設定の更新（Dashboard 設定画面） |
| `GET` | `/.well-known/agent-card.json` | レジストリ自身の Agent Card |
| `GET` | `/.well-known/agent.json` | レジストリの Agent Card（旧パス） |

---

## 設定

### オペレーターのフラグ

| フラグ | デフォルト | 説明 |
|---|---|---|
| `--metrics-bind-address` | `:8080` | Prometheus メトリクスエンドポイント |
| `--health-probe-bind-address` | `:8081` | ヘルス/レディネスプローブエンドポイント |
| `--registry-api-bind-address` | `:8082` | レジストリ検出 API |
| `--leader-elect` | `false` | Leader Election の有効化 |
| `--leader-election-id` | `a2a-registry` | Leader Election の識別子 |

### 環境変数

| 変数 | デフォルト | 説明 |
|---|---|---|
| `ENABLE_WEBHOOKS` | (有効) | `"false"` に設定するとアドミッション Webhook を無効化 |

---

## 監視

### Prometheus メトリクス

すべてのメトリクスは `a2a_registry_` プレフィックスでポート `:8080` に公開されます。

| メトリクス | タイプ | ラベル | 説明 |
|---|---|---|---|
| `a2a_registry_agent_count` | GaugeVec | `phase`, `health` | フェーズとヘルス状態別のエージェント数 |
| `a2a_registry_health_check_duration_seconds` | Histogram | — | エージェントヘルスチェック遅延 |
| `a2a_registry_health_check_failures_total` | Counter | — | ヘルスチェック失敗の総数 |
| `a2a_registry_agent_card_fetch_errors_total` | Counter | — | エージェントカード取得エラーの総数 |
| `a2a_registry_registrations_total` | Counter | — | エージェント登録の総数 |
| `a2a_registry_deregistrations_total` | Counter | — | エージェント登録解除の総数 |
| `a2a_registry_api_request_duration_seconds` | HistogramVec | `endpoint`, `method` | レジストリ API リクエスト遅延 |

### Kubernetes Events

状態遷移時に Kubernetes Event が発行され、`kubectl describe a2aa <name>` で確認できます：

| イベント | タイプ | 意味 |
|---|---|---|
| `FinalizerRemoved` | Normal | エージェント削除完了 |
| `HealthCheckRecovered` | Normal | エージェントが Error/Unreachable から Ready に復旧 |
| `CardMismatch` | Warning | 取得したカードが spec と一致しない |
| `HealthCheckFailed` | Warning | 単一のヘルスチェック失敗 |
| `AgentUnreachable` | Warning | 障害閾値を超えて Unreachable とマーク |
| `AgentPruned` | Normal | 7 日経過したエージェントの自動クリーンアップ |

---

## 開発

```bash
make build          # バイナリのビルド
make run            # ローカル実行（クラスタ外）
make test           # テストの実行（envtest が必要）
make generate manifests  # CRD マニフェストと DeepCopy メソッドの生成
make fmt            # コードのフォーマット
make vet            # コードの静的解析

# Docker イメージのビルドとプッシュ
export IMG=your-registry/a2a-registry:latest
make docker-build
make docker-push
```

### プロジェクト構造

```
.
├── api/v1/                    # CRD 型定義 + Webhook
├── cmd/manager/               # オペレーターのエントリポイント
├── config/
│   ├── crd/bases/             # 生成された CRD YAML マニフェスト
│   ├── rbac/                  # RBAC ロールとバインディング
│   ├── manager/               # Deployment マニフェスト
│   ├── default/               # トップレベル kustomization
│   └── samples/               # サンプル CR
├── controllers/               # 調整ロジック
├── internal/
│   ├── healthcheck/           # エージェントヘルスチェック
│   ├── metrics/               # Prometheus メトリクス定義
│   └── registry/              # API サーバー + ハンドラ + エージェントカードリゾルバ
├── examples/hello-agent/      # サンプル A2A エージェント
├── deploy/helm/a2a-registry/  # Helm チャート
├── vendor/                    # ベンダリングされた依存関係
├── Dockerfile                 # マルチステージコンテナビルド
└── Makefile                   # ビルドとデプロイのターゲット
```

---

## サンプル

[`examples/hello-agent/`](examples/hello-agent/) ディレクトリには完全に機能する A2A エージェントの実装が含まれています：

- `/.well-known/agent-card.json` で Agent Card を提供
- `/invoke` で JSON-RPC 呼び出しを受け付け
- 完全な `deploy.yaml`（Namespace、Deployment、Service、A2AAgent CR）を含む

```bash
cd examples/hello-agent
kubectl apply -f deploy.yaml
```

---

## 関連プロジェクト

- [A2A プロトコル仕様](https://github.com/google/A2A) — エージェント間通信のオープン標準
- [a2a-go](https://github.com/a2aproject/a2a-go) — A2A プロトコルの Go SDK
- [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) — Kubernetes オペレーター構築のためのフレームワーク

---

## ライセンス

本プロジェクトは Apache License 2.0 の下でライセンスされています。詳細は [LICENSE](LICENSE) をご参照ください。
