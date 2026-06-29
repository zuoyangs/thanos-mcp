# Thanos MCP Tools

本文档详细介绍 thanos-mcp 提供的所有工具，包括功能说明、参数定义、使用场景和返回数据结构。

## 目录

- [查询类工具](#查询类工具)
  - [query](#query)
  - [query_range](#query_range)
- [性能/健康类工具](#性能健康类工具)
  - [get_targets](#get_targets)
  - [get_rules](#get_rules)
  - [get_alerts](#get_alerts)
- [运维辅助类工具](#运维辅助类工具)
  - [get_cluster_info](#get_cluster_info)
  - [get_status](#get_status)
  - [get_stores](#get_stores)
- [高基数分析工具](#高基数分析工具)
  - [get_cardinality_analysis](#get_cardinality_analysis)

---

## 查询类工具

### query

执行即时 PromQL 查询，获取某一时刻的指标当前值。

**功能说明**：
- 对 Thanos/Prometheus 执行即时查询
- 必须包含 `bi=` 或 `cluster=` 标签过滤
- 返回符合条件的时间序列及其当前值

**参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| query | string | 是 | PromQL 查询字符串，必须包含 bi 或 cluster 标签 |

**使用场景**：
- 获取当前指标值
- 检查某个指标是否存在
- 查询特定集群或业务线的即时数据

**示例**：
```json
{
  "query": "up{cluster=\"ali-prod-ack\"}"
}
```

**返回结构**：
```json
{
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [
      {
        "metric": {
          "__name__": "up",
          "cluster": "ali-prod-ack",
          "job": "prometheus"
        },
        "value": [1704067200, "1"]
      }
    ]
  }
}
```

---

### query_range

执行范围 PromQL 查询，获取一段时间内的指标趋势数据。

**功能说明**：
- 对 Thanos/Prometheus 执行范围查询
- 必须包含 `bi=` 或 `cluster=` 或 `kube_cluster_alias=`标签过滤
- 返回指定时间范围内的时序数据点

**参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| query | string | 是 | PromQL 查询字符串 |
| start | number | 是 | 起始时间 Unix 时间戳（秒） |
| end | number | 是 | 结束时间 Unix 时间戳（秒） |
| step | string | 是 | 查询步长（如 '15s', '1m', '5m', '1h'） |

**使用场景**：
- 获取指标历史趋势
- 绘制监控图表
- 分析时间序列变化

**示例**：
```json
{
  "query": "rate(http_requests_total{bi=\"my-service\"}[5m])",
  "start": 1704067200,
  "end": 1704153600,
  "step": "5m"
}
```

**返回结构**：
```json
{
  "status": "success",
  "data": {
    "resultType": "matrix",
    "result": [
      {
        "metric": {
          "__name__": "http_requests_total",
          "bi": "my-service"
        },
        "values": [
          [1704067200, "100"],
          [1704067500, "105"],
          [1704067800, "110"]
        ]
      }
    ]
  }
}
```

---

## 性能/健康类工具

### get_targets

获取 Prometheus 采集目标状态，列出所有 active/dropped 目标及其健康状态。

**功能说明**：
- 查询 `/api/v1/targets` 端点
- 返回采集目标列表及其状态（up/down）
- 支持按 `bi`、`cluster`、`cluster_alias`、`job` 过滤
- 返回健康状态统计摘要

**参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| state | string | 否 | 目标状态过滤: `active`, `dropped`, `any`（默认返回全部） |
| bi | string | 否 | 按 bi 标签精确过滤 |
| cluster | string | 否 | 按 cluster 标签精确过滤 |
| cluster_alias | string | 否 | 按 cluster_alias 标签精确过滤（人读集群名） |
| job | string | 否 | 按 job 名称关键字过滤（模糊匹配） |
| limit | number | 否 | 限制返回数量，避免大数据量超时 |

**使用场景**：
- 监控采集链路健康
- 排查数据缺失问题
- 检查特定 job 的采集状态

**示例**：
```json
{
  "bi": "ali-prod-ack",
  "state": "active",
  "limit": 50
}
```

**返回结构**：
```json
{
  "active_targets": [...],
  "active_count": 45,
  "health_summary": {
    "up": 43,
    "down": 2,
    "unknown": 0
  },
  "filters": {
    "bi": "ali-prod-ack"
  }
}
```

---

### get_rules

获取告警规则和录制规则，列出所有规则组及其规则。

**功能说明**：
- 查询 `/api/v1/rules` 端点
- 返回规则组和规则列表
- 支持按 `bi`、`cluster`、`cluster_alias`、`group` 过滤
- 统计告警规则/录制规则数量

**参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| type | string | 否 | 规则类型过滤: `alert`（告警规则）, `record`（录制规则） |
| bi | string | 否 | 按 bi 标签精确过滤 |
| cluster | string | 否 | 按 cluster 标签精确过滤 |
| cluster_alias | string | 否 | 按 cluster_alias 标签精确过滤（人读集群名） |
| group | string | 否 | 按规则组名称关键字过滤（模糊匹配） |
| limit | number | 否 | 限制返回规则数量 |

**使用场景**：
- 审计告警配置
- 排查告警不触发问题
- 检查录制规则配置

**示例**：
```json
{
  "type": "alert",
  "cluster": "ali-prod-ack"
}
```

**返回结构**：
```json
{
  "groups": [...],
  "group_count": 5,
  "total_rules": 20,
  "alert_rules": 15,
  "record_rules": 5,
  "filters": {
    "type": "alert",
    "cluster": "ali-prod-ack"
  }
}
```

---

### get_alerts

获取当前活跃告警，列出所有 firing/pending 状态的告警。

**功能说明**：
- 从 `/api/v1/rules?type=alert` 提取 firing/pending 状态的告警
- 支持按 severity 过滤
- 返回告警状态统计摘要

**参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| severity | string | 否 | 按告警级别过滤（如 `critical`, `warning`, `info`） |

**使用场景**：
- 实时告警巡检
- 故障排查
- 告警统计分析

**示例**：
```json
{
  "severity": "critical"
}
```

**返回结构**：
```json
{
  "alerts": [
    {
      "name": "HighMemoryUsage",
      "state": "firing",
      "labels": {
        "severity": "critical",
        "cluster": "ali-prod-ack"
      },
      "annotations": {
        "summary": "Memory usage above 90%"
      },
      "group": "node-alerts",
      "file": "/etc/prometheus/rules/node.yml"
    }
  ],
  "total": 3,
  "state_summary": {
    "firing": 2,
    "pending": 1
  }
}
```

---

## 运维辅助类工具

### get_cluster_info

获取 Thanos/Prometheus 集群综合信息。

**功能说明**：
- 返回连接配置信息（endpoint, timeout, transport）
- 返回运行时信息（存储保留期、WAL 大小、GOMAXPROCS 等）
- 返回启动参数 flags

**参数**：无

**使用场景**：
- 集群巡检
- 运维排障
- 配置确认

**返回结构**：
```json
{
  "thanos_endpoint": "https://thanos-query.<your-domain>.com",
  "thanos_timeout": "30s",
  "transport": "streamable-http",
  "auth_enabled": false,
  "auth_users": 0,
  "server_time": "2024-01-01T12:00:00Z",
  "server_time_utc": "2024-01-01T12:00:00Z",
  "server_unix": 1704110400,
  "runtime_info": {
    "storageRetention": "15d",
    "tsdbMaxBytes": 1073741824
  },
  "flags": {
    "query.timeout": "2m",
    "query.max-samples": 50000000
  }
}
```

---

### get_status

获取 Thanos/Prometheus 运行状态。

**功能说明**：
- 返回 Runtime Information（启动时间、工作目录、Goroutines 数等）
- 返回 Build Information（版本号、Git revision、构建时间等）

**参数**：无

**使用场景**：
- 版本确认
- 运行状态巡检
- 性能分析

**返回结构**：
```json
{
  "runtime_information": {
    "startTime": "2024-01-01T00:00:00Z",
    "workingDirectory": "/prometheus",
    "goroutineCount": 150,
    "GOMAXPROCS": 8,
    "GOGC": "100"
  },
  "build_information": {
    "version": "2.45.0",
    "revision": "abc123",
    "branch": "main",
    "buildUser": "root@builder",
    "buildDate": "2023-12-01",
    "goVersion": "1.21.0"
  }
}
```

---

### get_stores

获取 Thanos StoreAPI 组件列表，这是最重要的运维工具之一。

**功能说明**：
- 查询 `/api/v1/stores` 端点
- 返回所有注册到 Thanos Query 的存储端点（sidecar、store gateway、ruler、receive 等）
- 返回每个 store 的类型、地址、标签集（external_labels）、时间范围、健康状态
- **返回 `labelsets_mapping` 字段，提供 `cluster ↔ bi ↔ location ↔ kube_cluster_alias` 等映射关系**

**参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| type | string | 否 | 按 store 类型过滤（如 `sidecar`, `store`, `rule`, `receive`） |

**使用场景**：
- 排查 sidecar 掉线
- 排查 store gateway 异常
- 检查数据源覆盖范围
- **获取集群别名映射关系**（替代硬编码映射表）

**示例**：
```json
{}
```

或按类型过滤：
```json
{
  "type": "sidecar"
}
```

**返回结构**：
```json
{
  "stores": [
    {
      "storeType": "sidecar",
      "addr": "10.0.0.1:10901",
      "labels": {
        "bi": "ali-prod-ack",
        "cluster": "ali-prod-ack",
        "kube_cluster_alias": "ali-prod-ack",
        "location": "Ali"
      },
      "minTime": 1704000000,
      "maxTime": 1704110400,
      "lastError": ""
    }
  ],
  "total": 10,
  "type_summary": {
    "sidecar": 5,
    "store": 5
  },
  "health_summary": {
    "healthy": 10,
    "unhealthy": 0
  },
  "labelsets_mapping": {
    "ali-prod-ack": {
      "bi": "ali-prod-ack",
      "cluster": "ali-prod-ack",
      "kube_cluster_alias": "ali-prod-ack",
      "kube_cluster_name": "ali-prod-ack",
      "location": "Ali",
      "prometheus": "monitoring/k8s",
      "prometheus_replica": "prometheus-k8s-0"
    },
    "z-prod-tke": {
      "bi": "z-prod-tke",
      "cluster": "ninja-cloud-prod",
      "cluster_alias": "z-prod-tke",
      "location": "Tencent",
      "prometheus": "monitoring/prometheus"
    }
  }
}
```

#### labelsets_mapping 字段说明

`labelsets_mapping` 是从每个 store 的 `external_labels` 中提取的映射关系，用于解决以下问题：

1. **cluster label 命名规范化**：用户习惯用人读别名（如 `z-prod-tke`），但 PromQL 必须用真实值（如 `ninja-cloud-prod`）
2. **避免硬编码映射表**：Agent 不需要在知识卡片中维护 cluster alias 映射表
3. **动态发现**：新增集群时无需更新配置，自动从 stores 中发现

映射关系示例：
- `bi` → `cluster`：业务线到集群的映射
- `bi` → `location`：业务线到地域的映射
- `cluster` → `kube_cluster_alias`：集群到人读别名的映射

---

## 高基数分析工具

### get_cardinality_analysis

针对指定 bi 或 cluster 的 Prometheus 实例进行高基数分析。

**功能说明**：
- 直接查询 Prometheus 的 `/api/v1/status/tsdb` 端点
- 获取 Top N 高基数指标、高基数标签、标签值对分布
- 结合 PromQL 查询按 job 维度统计序列数分布
- 动态分析高基数问题，不预设特定指标前缀

**参数**：

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| prometheus_endpoint | string | 是 | 目标 Prometheus 实例地址（如 `http://prometheus:9090`） |
| bi | string | 否 | BI 标签值，用于限定分析范围（与 cluster 二选一必填） |
| cluster | string | 否 | 集群标签值，用于限定分析范围（与 bi 二选一必填） |
| limit | number | 否 | Top N 返回数量，默认 20 |

**使用场景**：
- 排查序列膨胀
- 优化 relabeling 策略
- 降低存储和查询成本

**示例**：
```json
{
  "prometheus_endpoint": "http://prometheus-k8s-0:9090",
  "bi": "ali-prod-ack",
  "limit": 20
}
```

**返回结构**：
```json
{
  "prometheus_endpoint": "http://prometheus-k8s-0:9090",
  "head_stats": {
    "numSeries": 1000000,
    "numLabelPairs": 5000000,
    "chunkCount": 2000000
  },
  "series_count_by_metric_name": [
    {"name": "http_requests_total", "value": 50000},
    {"name": "container_cpu_usage", "value": 30000}
  ],
  "label_value_count_by_label_name": [
    {"name": "pod", "value": 10000},
    {"name": "container", "value": 5000}
  ],
  "series_count_by_label_value_pair": [
    {"name": "job=prometheus", "value": 5000},
    {"name": "job=node-exporter", "value": 3000}
  ],
  "job_series_distribution": {
    "prometheus": 5000,
    "node-exporter": 3000,
    "kubelet": 2000
  }
}
```

---

## 工具使用最佳实践

### 1. 集群名称转换

使用 `get_stores` 的 `labelsets_mapping` 字段进行集群名称转换：

```
用户输入: "z-prod-tke 集群的 CPU 使用率"
1. 调用 get_stores 获取 labelsets_mapping
2. 查找 bi="z-prod-tke" 或 kube_cluster_alias="z-prod-tke"
3. 获取对应的 cluster="ninja-cloud-prod"
4. 执行 query: rate(container_cpu_usage_seconds_total{cluster="ninja-cloud-prod"}[5m])
```

### 2. 标签过滤要求

所有查询类工具（`query`、`query_range`）必须包含 `bi=` 或 `cluster=` 标签过滤，这是为了避免全量扫描导致查询超时。

### 3. 高基数排查流程

1. 调用 `get_stores` 获取 sidecar 地址和 cluster 信息
2. 调用 `get_cardinality_analysis` 分析目标 Prometheus 的高基数指标
3. 根据 Top N 指标和标签定位问题
4. 优化 relabeling 配置或调整指标采集策略

### 4. 告警排查流程

1. 调用 `get_alerts` 查看当前活跃告警
2. 调用 `get_rules` 检查告警规则配置
3. 调用 `get_targets` 检查相关 job 的采集状态
4. 使用 `query` 或 `query_range` 分析指标数据

---

## 源码文件说明

| 文件 | 功能 |
|------|------|
| `tools.go` | 工具定义，包含所有工具的 name、description、inputSchema |
| `handler.go` | 工具处理器入口，路由分发到具体处理函数 |
| `query.go` | 查询类工具实现（query、query_range） |
| `health.go` | 健康类工具实现（get_targets、get_rules、get_alerts） |
| `ops.go` | 运维类工具实现（get_cluster_info、get_status、get_stores） |
| `cardinality.go` | 高基数分析工具实现 |
