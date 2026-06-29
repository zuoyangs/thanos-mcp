<<<<<<< HEAD
# thanos-mcp
Thanos/Prometheus 指标查询 MCP Server - 支持 PromQL 即时/范围查询、高基数分析、告警规则审计、采集目标健康检查，提供 stdio/HTTP/Streamable HTTP 多种传输模式。
=======
# Thanos MCP Server

基于 [MCP (Model Context Protocol)](https://modelcontextprotocol.io/) 的 Thanos/Prometheus 只读查询服务器，支持 PromQL 即时/范围查询、高基数分析（cardinality_analysis）、采集目标健康检查、告警规则审计、Store 组件巡检等运维能力，提供 stdio/HTTP/Streamable HTTP 多种传输模式及 Bearer Token/Basic Auth 认证。

## 功能特性

- PromQL 即时查询（`query`）和范围查询（`query_range`）
- 高基数分析（`get-cardinality-analysis`）：TSDB Status + Top N 序列数统计
- 采集目标健康检查、告警规则审计、Store 组件巡检
- 三种传输模式：`stdio`、`streamable-http`、`http`
- 内置鉴权：Basic Auth / Bearer Token（基于 [go-mcp-common](https://github.com/zuoyangs/go-mcp-common)）
- 查询安全校验：强制要求 `bi=` 或 `cluster=` 或 `kube_cluster_alias=` 标签过滤（三选一），防止全量查询
- 多级日志系统：debug / info / error / access 分文件记录
- Docker 多阶段构建 + k8s 部署支持

## 项目结构

```
thanos-mcp/
├── main.go                          # 入口：flag 解析、加载配置、初始化日志、启动传输层
├── config/config.go                 # 配置加载（viper + flag 指定路径），集成 serverauth
├── thanos/
│   ├── client.go                    # Thanos HTTP 客户端
│   └── types.go                     # 查询结果类型定义
├── tools/handler.go                 # MCP 工具定义与调用处理
├── transport/
│   ├── jsonrpc.go                   # JSON-RPC 公共类型与请求路由
│   ├── stdio.go                     # stdio 传输（适配 Claude Desktop 等）
│   ├── http.go                      # HTTP 传输
│   └── streamable_http.go           # Streamable HTTP 传输（SSE + JSON-RPC）
├── utils/logger.go                  # 多级日志器
├── etc/config.yaml                  # 配置文件
├── k8s/                             # Kubernetes 部署清单
├── Dockerfile                       # 多阶段构建
└── Makefile                         # 构建 / 运行 / 推送
```

## MCP 工具

### PromQL 查询

| 工具 | 说明 |
|------|------|
| `query` | 执行 PromQL 即时查询，返回指标当前值。必须携带 `bi=` 或 `cluster=` 或 `kube_cluster_alias=` 标签实现租户/集群隔离，适用于实时状态核查 |
| `query_range` | 执行 PromQL 区间查询，返回历史趋势数据。支持指定 start/end/step（步长），用于容量分析、故障回溯、趋势预测 |

### 采集目标与规则

| 工具 | 说明 |
|------|------|
| `get-targets` | 获取 Prometheus 采集目标全生命周期状态（active/dropped），包含健康状态、失败原因、最后采集时间。支持按 scrape_pool/bi/cluster/kube_cluster_alias/job 过滤 |
| `get-rules` | 获取告警规则与录制规则全量配置，包含规则组、表达式、执行状态、错误信息。支持按类型/规则组/文件/集群/租户过滤 |
| `get-alerts` | 获取当前活跃告警清单（firing/pending），包含告警来源、标签、触发时间、持续时长。支持按告警级别过滤 |

### 集群与存储

| 工具 | 说明 |
|------|------|
| `get-cluster-info` | 获取 Prometheus/Thanos 集群核心运行时信息：存储策略、数据保留周期、WAL 状态、资源限制、启动配置 |
| `get-status` | 获取系统运行态与构建信息：版本、启动时间、Goroutines、GC 配置、资源使用概况 |
| `get-stores` | 获取 Thanos StoreAPI 全量注册节点：sidecar、store gateway、ruler、receive 等组件状态、时间覆盖范围、外部标签与健康状态 |

### 高基数分析

| 工具 | 说明 |
|------|------|
| `get-cardinality-analysis` | Prometheus 高基数智能深度分析，基于 TSDB Status 实时采集 TopN 高基数指标、标签爆炸、标签值分布、内存占用与 job 级序列数。用于定位时序膨胀、优化采集策略、缓解 OOM |

## 标签过滤说明

所有查询工具（`query`、`query_range`、`get-targets`、`get-rules`、`get-cardinality-analysis`）都支持通过标签进行租户/集群隔离。

### 支持的标签

| 标签 | 说明 | 示例 |
|------|------|------|
| `bi` | 业务线标签 | `bi="analytics"` |
| `cluster` | 集群标签 | `cluster="prod"` |
| `kube_cluster_alias` | Kubernetes 集群别名 | `kube_cluster_alias="prod-k8s"` |

### 过滤关系

三个标签为 **OR 关系**，任选其一即可：

```promql
# 示例 1：按 bi 过滤
up{bi="analytics"}

# 示例 2：按 cluster 过滤
up{cluster="prod"}

# 示例 3：按 kube_cluster_alias 过滤
up{kube_cluster_alias="prod-k8s"}

# 示例 4：多个标签组合（仍为 OR 关系）
up{bi="analytics", cluster="prod"}
```

### 工具级别的标签过滤

- **`get-targets`**、**`get-rules`**：支持通过参数指定标签过滤，至少需要提供一个过滤条件（`scrape_pool` / `job` / `bi` / `cluster` / `kube_cluster_alias` / `cluster_alias`）
- **`get-cardinality-analysis`**：必须提供 `bi`、`cluster` 或 `kube_cluster_alias` 之一，用于限定分析范围

## 快速开始

### 1. 配置

1. 复制示例配置文件：

```bash
cp etc/config.yaml.example etc/config.yaml
```

2. 编辑 `etc/config.yaml`：

```yaml
thanos:
  endpoint: "https://thanos-query.example.com"
  timeout: "30s"

mcp:
  transport: "streamable-http"  # stdio | streamable-http | http
  port: 8080

auth:
  enabled: true
  token: "your-bearer-token"
  users:
    - username: admin
      password: s3cret
      token: "admin-bearer-token"

logging:
  dir: "logs"
  level: "info"
```

### 2. 本地运行

```bash
# 构建
go build -o thanos-mcp .

# 使用 etc/config.yaml 启动（默认 streamable-http 模式，端口 8080）
./thanos-mcp -config etc/config.yaml

# 也可以用 go run 直接运行
go run . -config etc/config.yaml

# 指定其他配置文件
./thanos-mcp -config /path/to/config.yaml

# 通过环境变量覆盖为 stdio 模式
THANOS_ENDPOINT=https://thanos-query.example.com MCP_TRANSPORT=stdio ./thanos-mcp -config etc/config.yaml
```

### 3. MCP 客户端配置

#### Claude Desktop / Cursor（stdio 模式）

```json
{
  "mcpServers": {
    "thanos-mcp": {
      "command": "/path/to/thanos-mcp",
      "args": ["-config", "/path/to/etc/config.yaml"],
      "env": {
        "MCP_TRANSPORT": "stdio"
      }
    }
  }
}
```

#### Cherry Studio / HTTP 客户端（streamable-http 模式）

```
URL: http://<host>:8080/mcp
Authorization: Bearer <your-token>
```

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `THANOS_ENDPOINT` | Thanos Query 地址 | 配置文件中的值 |
| `MCP_TRANSPORT` | 传输模式 | `stdio` |
| `MCP_PORT` | HTTP 监听端口 | `8080` |
| `MCP_AUTH_ENABLED` | 是否启用鉴权 | 配置文件中的值 |
| `MCP_AUTH_TOKEN` | 全局 Bearer Token | 配置文件中的值 |
| `MCP_LOG_DIR` | 日志目录 | `./logs` |
| `MCP_LOG_LEVEL` | 日志级别 | `info` |

环境变量优先级高于配置文件。

## 鉴权

基于 [go-mcp-common/serverauth](https://github.com/zuoyangs/go-mcp-common) 实现：

| 方式 | Header 格式 | 说明 |
|------|------------|------|
| Basic Auth | `Basic <base64(user:pass)>` | 匹配 `users` 列表 |
| Bearer Token | `Bearer <token>` | 匹配全局 `token` 或用户级 `token` |
| 无认证 | — | `enabled: false` 时放行所有请求 |

## Docker

```bash
make build    # 构建镜像
make run      # 运行（streamable-http 模式）
make push     # 推送到 Harbor
```

## Kubernetes 部署

```bash
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
```

部署后通过 NodePort 或 Ingress 访问 `http://<node>:<port>/mcp`。

## 日志

日志按级别分文件输出到 `logs/` 目录，按日期轮转：

| 文件 | 内容 |
|------|------|
| `debug-YYYY-MM-DD.log` | 调试信息 |
| `info-YYYY-MM-DD.log` | 运行信息 + 警告 |
| `error-YYYY-MM-DD.log` | 错误信息 |
| `access-YYYY-MM-DD.log` | 访问日志（IP、用户、工具调用） |

## License

MIT
>>>>>>> c27b64c (feat: 初始化项目 - Thanos 指标查询 MCP Server)
