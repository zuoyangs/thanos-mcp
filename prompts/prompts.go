// Package prompts 提供 Thanos MCP 的 Prompt 定义。
// 由于 thanos-mcp 使用自定义 JSON-RPC transport，prompts 以静态数据结构返回。
package prompts

import "fmt"

// PromptDef 定义一个 Prompt
type PromptDef struct {
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Arguments   []ArgDef   `json:"arguments,omitempty"`
}

// ArgDef 定义 Prompt 参数
type ArgDef struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// MessageDef 定义 Prompt 消息
type MessageDef struct {
	Role    string     `json:"role"`
	Content ContentDef `json:"content"`
}

// ContentDef 定义消息内容
type ContentDef struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// GetPromptResult 定义 GetPrompt 返回
type GetPromptResult struct {
	Description string       `json:"description,omitempty"`
	Messages    []MessageDef `json:"messages"`
}

// GetAllPrompts 返回所有 Prompt 定义
func GetAllPrompts() []PromptDef {
	return []PromptDef{
		{
			Name:        "thanos_cluster_inspection",
			Description: "Thanos/Prometheus 集群巡检：检查采集目标健康状态、告警规则、活跃告警、Store 组件健康，输出巡检报告。",
		},
		{
			Name:        "thanos_alert_analysis",
			Description: "活跃告警分析：获取当前所有 firing/pending 告警，按严重级别分类，分析告警原因并给出处理建议。",
			Arguments: []ArgDef{
				{Name: "severity", Description: "告警级别过滤（可选）：critical、warning、info"},
			},
		},
		{
			Name:        "thanos_target_health_check",
			Description: "采集目标健康检查：检查所有 Prometheus 采集目标的 up/down 状态，找出不健康的目标，分析可能的原因。",
			Arguments: []ArgDef{
				{Name: "job", Description: "按 job 名称过滤（可选）"},
			},
		},
		{
			Name:        "thanos_high_cardinality_analysis",
			Description: "高基数指标分析：针对指定 bi 或 cluster 的 Prometheus 实例，通过 TSDB Status API 和 PromQL 查询找出时间序列数最多的指标，分析标签基数，给出优化建议以降低存储和查询成本。支持精确定位到具体 Prometheus 实例（prometheus/prometheus_replica）。",
			Arguments: []ArgDef{
				{Name: "bi", Description: "BI 标签值（与 cluster 二选一必填）"},
				{Name: "cluster", Description: "集群标签值（与 bi 二选一必填）"},
				{Name: "prometheus", Description: "可选，Prometheus 实例标签（如 monitoring/k8s）"},
				{Name: "prometheus_replica", Description: "可选，Prometheus 副本标签（如 prometheus-k8s-0）"},
			},
		},
		{
			Name:        "thanos_store_health_check",
			Description: "Thanos Store 组件健康检查：检查所有 Sidecar、Store Gateway、Ruler、Receive 等组件的连接状态和时间范围覆盖。",
		},
		{
			Name:        "thanos_promql_helper",
			Description: "PromQL 查询助手：根据自然语言描述自动构建 PromQL 查询并执行。注意查询必须包含 cluster= 或 bi= 标签过滤。",
			Arguments: []ArgDef{
				{Name: "description", Description: "查询需求的自然语言描述", Required: true},
			},
		},
		{
			Name:        "thanos_cost_analysis",
			Description: "Prometheus/Thanos 存储成本分析：通过 PromQL 分析时间序列数、高基数指标、按 job/指标名的序列数分布，评估存储成本和优化空间。",
		},
		{
			Name:        "thanos_capacity_planning",
			Description: "Prometheus/Thanos 容量规划：基于当前时间序列增长趋势、采集目标数量，预测存储增长和查询性能影响。",
		},
		{
			Name:        "thanos_query_performance",
			Description: "Thanos 查询性能分析：评估查询延迟、Store 组件响应时间、高基数查询的影响，给出查询优化建议。",
			Arguments: []ArgDef{
				{Name: "slow_query", Description: "慢查询的 PromQL 表达式（可选，用于针对性分析）"},
			},
		},
	}
}

// HandleGetPrompt 处理 prompts/get 请求
func HandleGetPrompt(name string, args map[string]string) *GetPromptResult {
	switch name {
	case "thanos_cluster_inspection":
		return handleClusterInspection()
	case "thanos_alert_analysis":
		return handleAlertAnalysis(args["severity"])
	case "thanos_target_health_check":
		return handleTargetHealthCheck(args["job"])
	case "thanos_high_cardinality_analysis":
		return handleHighCardinalityAnalysis(args["bi"], args["cluster"], args["prometheus"], args["prometheus_replica"])
	case "thanos_store_health_check":
		return handleStoreHealthCheck()
	case "thanos_promql_helper":
		return handlePromQLHelper(args["description"])
	case "thanos_cost_analysis":
		return handleThanosCostAnalysis()
	case "thanos_capacity_planning":
		return handleThanosCapacityPlanning()
	case "thanos_query_performance":
		return handleThanosQueryPerformance(args["slow_query"])
	default:
		return &GetPromptResult{Messages: []MessageDef{{Role: "user", Content: ContentDef{Type: "text", Text: "未知 Prompt: " + name}}}}
	}
}

func handleClusterInspection() *GetPromptResult {
	return &GetPromptResult{
		Description: "Thanos/Prometheus 集群巡检",
		Messages: []MessageDef{{Role: "user", Content: ContentDef{Type: "text", Text: `请对 Thanos/Prometheus 集群进行全面巡检。

## 巡检步骤

**第一步：集群信息**
调用 get_cluster_info 获取 Thanos 连接配置和运行时信息

**第二步：采集目标健康**
调用 get_targets 检查所有采集目标的 up/down 状态
统计健康/不健康目标数量

**第三步：活跃告警**
调用 get_alerts 获取当前所有活跃告警
按 severity 分类统计

**第四步：Store 组件**
调用 get_stores 检查所有 Store 组件连接状态

## 输出格式
## Thanos 集群巡检报告
| 指标 | 值 | 状态 |
|------|-----|------|
| 采集目标 UP | x/y | ✅/❌ |
| 活跃告警 | x (critical: y) | ✅/⚠️ |
| Store 组件 | x healthy | ✅/❌ |

### 不健康目标列表
### 活跃告警列表
### 优化建议`}}},
	}
}

func handleAlertAnalysis(severity string) *GetPromptResult {
	sevFilter := ""
	if severity != "" {
		sevFilter = "，按 severity=\"" + severity + "\" 过滤"
	}
	return &GetPromptResult{
		Description: "活跃告警分析",
		Messages: []MessageDef{{Role: "user", Content: ContentDef{Type: "text", Text: "请分析当前所有活跃告警" + sevFilter + `。

## 步骤
1. 调用 get_alerts 获取所有活跃告警
2. 按 severity 分组统计（critical/warning/info）
3. 对每个告警分析可能的原因
4. 给出处理优先级和建议

## 输出格式
### 告警统计
| 级别 | 数量 |
|------|------|
| critical | x |
| warning | y |

### 告警详情
按优先级排列，每个告警包含：名称、级别、持续时间、可能原因、处理建议`}}},
	}
}

func handleTargetHealthCheck(job string) *GetPromptResult {
	jobFilter := ""
	if job != "" {
		jobFilter = "，按 job=\"" + job + "\" 过滤"
	}
	return &GetPromptResult{
		Description: "采集目标健康检查",
		Messages: []MessageDef{{Role: "user", Content: ContentDef{Type: "text", Text: "请检查 Prometheus 采集目标的健康状态" + jobFilter + `。

## 步骤
1. 调用 get_targets(state="active") 获取所有活跃目标
2. 找出 health="down" 的目标
3. 分析不健康目标的可能原因（网络不通？服务挂了？端口错误？）

## 输出格式
### 健康统计
UP: x | DOWN: y | 总计: z

### 不健康目标列表
| Job | Instance | 最后抓取时间 | 错误信息 |
|-----|----------|------------|---------|`}}},
	}
}

func handleHighCardinalityAnalysis(bi, cluster, prometheus, prometheusReplica string) *GetPromptResult {
	// 构建目标描述
	targetDesc := ""
	if bi != "" {
		targetDesc += fmt.Sprintf(`bi="%s"`, bi)
	}
	if cluster != "" {
		if targetDesc != "" {
			targetDesc += ", "
		}
		targetDesc += fmt.Sprintf(`cluster="%s"`, cluster)
	}
	if prometheus != "" {
		if targetDesc != "" {
			targetDesc += ", "
		}
		targetDesc += fmt.Sprintf(`prometheus="%s"`, prometheus)
	}
	if prometheusReplica != "" {
		if targetDesc != "" {
			targetDesc += ", "
		}
		targetDesc += fmt.Sprintf(`prometheus_replica="%s"`, prometheusReplica)
	}
	if targetDesc == "" {
		targetDesc = "（未指定目标，请提供 bi 或 cluster 参数）"
	}

	// 构建工具调用参数提示
	toolArgs := ""
	if bi != "" {
		toolArgs += fmt.Sprintf(`, "bi": "%s"`, bi)
	}
	if cluster != "" {
		toolArgs += fmt.Sprintf(`, "cluster": "%s"`, cluster)
	}
	if prometheus != "" {
		toolArgs += fmt.Sprintf(`, "prometheus": "%s"`, prometheus)
	}
	if prometheusReplica != "" {
		toolArgs += fmt.Sprintf(`, "prometheus_replica": "%s"`, prometheusReplica)
	}
	if len(toolArgs) > 2 {
		toolArgs = toolArgs[2:] // 去掉开头的 ", "
	}

	return &GetPromptResult{
		Description: "高基数指标分析",
		Messages: []MessageDef{{Role: "user", Content: ContentDef{Type: "text", Text: fmt.Sprintf(`请对以下 Prometheus 实例进行高基数分析：
目标: %s

## 步骤

**第一步：调用 cardinality_analysis 工具**
调用 cardinality_analysis 工具，参数: {%s}
该工具会通过 TSDB Status API 获取：
- Head 统计（总序列数、标签对数、Chunk 数）
- Top N 高基数指标（按时间序列数排序）
- Top N 高基数标签（按标签值数量排序）
- Top N 标签内存占用
- Top N 标签值对（按序列数排序）
- 按 job 维度的序列数分布

**第二步：分析高基数根因**
根据返回数据分析：
1. 哪些指标的时间序列数异常偏高？
2. 哪些标签的基数（唯一值数量）过高？（如 pod、container_id、request_id 等）
3. 哪些 job 贡献了最多的序列数？
4. 是否存在不合理的标签值对组合导致序列膨胀？

**第三步：给出优化建议**

## 输出格式

### TSDB Head 概览
| 指标 | 值 |
|------|-----|
| 总时间序列数 | x |
| 总标签对数 | x |
| Chunk 数 | x |

### 高基数指标 Top 10
| 排名 | 指标名 | 时间序列数 | 占比 | 建议 |
|------|--------|-----------|------|------|

### 高基数标签 Top 10
| 排名 | 标签名 | 唯一值数量 | 内存占用 | 建议 |
|------|--------|-----------|---------|------|

### 按 Job 序列数分布
| Job | 序列数 | 占比 |
|-----|--------|------|

### 优化建议
1. 可以通过 metric_relabel_configs 降低基数的指标及具体 relabeling 配置示例
2. 应该 drop 的无用高基数标签
3. 预计优化后可减少的时间序列数和存储空间`, targetDesc, toolArgs)}}},
	}
}

func handleStoreHealthCheck() *GetPromptResult {
	return &GetPromptResult{
		Description: "Thanos Store 组件健康检查",
		Messages: []MessageDef{{Role: "user", Content: ContentDef{Type: "text", Text: `请检查所有 Thanos Store 组件的健康状态。

## 步骤
1. 调用 get_stores 获取所有 Store 组件
2. 按类型分组（sidecar/store/rule/receive）
3. 检查每个组件的连接状态和时间范围覆盖

## 输出格式
### Store 组件统计
| 类型 | 总数 | 健康 | 不健康 |
|------|------|------|--------|

### 不健康组件列表
| 类型 | 地址 | 错误信息 | 时间范围 |
|------|------|---------|---------|

### 时间范围覆盖分析
确认所有时间范围是否连续，是否有数据空洞`}}},
	}
}

func handlePromQLHelper(description string) *GetPromptResult {
	if description == "" {
		description = "（未提供描述）"
	}
	return &GetPromptResult{
		Description: "PromQL 查询",
		Messages: []MessageDef{{Role: "user", Content: ContentDef{Type: "text", Text: "请根据以下需求构建并执行 PromQL 查询：\n" + description + `

## 要求
1. 构建合适的 PromQL 查询语句
2. 查询必须包含 cluster= 或 bi= 标签过滤（这是强制要求）
3. 调用 query 工具执行查询
4. 以易读的格式展示结果
5. 如果结果为空，给出可能的原因和调整建议`}}},
	}
}

func handleThanosCostAnalysis() *GetPromptResult {
	return &GetPromptResult{
		Description: "Prometheus/Thanos 存储成本分析",
		Messages: []MessageDef{{Role: "user", Content: ContentDef{Type: "text", Text: `请分析 Prometheus/Thanos 的存储成本。

## 分析步骤

**第一步：运行时与存储状态**
调用 get_status 获取运行时信息（存储保留期等）

**第二步：采集目标统计**
调用 get_targets 获取所有采集目标
统计各 job 的目标数量

**第三步：高基数指标分析**
通过 query 工具执行 PromQL 查询分析各指标的时间序列数分布

## 输出格式

### 存储概览
| 指标 | 值 |
|------|-----|
| 采集目标数 | x |
| 存储保留期 | x |

### 按 Job 的目标数分布
| Job | 目标数 | 估算序列数 |

### 成本优化建议
1. 可以通过 relabeling 降低基数的指标
2. 可以减少采集频率的 job
3. 可以删除的无用指标
4. 预计优化后可减少的序列数和存储空间`}}},
	}
}

func handleThanosCapacityPlanning() *GetPromptResult {
	return &GetPromptResult{
		Description: "Prometheus/Thanos 容量规划",
		Messages: []MessageDef{{Role: "user", Content: ContentDef{Type: "text", Text: `请对 Prometheus/Thanos 进行容量规划分析。

## 分析步骤

**第一步：当前运行状态**
调用 get_status 获取运行时信息（存储保留期等）

**第二步：采集目标增长**
调用 get_targets 获取当前采集目标数量
评估新增服务/目标对序列数的影响

**第三步：Store 组件覆盖**
调用 get_stores 检查各 Store 组件的时间范围覆盖
评估长期存储的容量需求

## 输出格式

### 当前容量状态
| 指标 | 值 |
|------|-----|
| 采集目标数 | x |
| 存储保留期 | x |
| Store 组件数 | x |

### 增长预测
- 当前采集目标增长趋势
- 预计 30/90/180 天后的容量需求
- 对查询性能的影响评估

### 容量优化方案
| 方案 | 操作 | 效果 | 风险 |
|------|------|------|------|
| A: 降低高基数指标 | relabeling | 减少 x% 序列 | 低 |
| B: 调整采集频率 | 非关键 job 降频 | 减少 x% 采样 | 低 |
| C: 缩短保留期 | 调整 retention | 释放存储 | 中 |
| D: 扩容存储 | 增加 Store Gateway 磁盘 | 延长保留 | 低 |`}}},
	}
}

func handleThanosQueryPerformance(slowQuery string) *GetPromptResult {
	queryHint := ""
	if slowQuery != "" {
		queryHint = "\n\n用户提供的慢查询：`" + slowQuery + "`\n请重点分析这个查询的性能问题。"
	}
	return &GetPromptResult{
		Description: "Thanos 查询性能分析",
		Messages: []MessageDef{{Role: "user", Content: ContentDef{Type: "text", Text: `请分析 Thanos 的查询性能。` + queryHint + `

## 分析步骤

**第一步：Store 组件健康**
调用 get_stores 检查所有 Store 组件的连接状态和响应能力
不健康的 Store 会导致查询超时或数据不完整

**第二步：运行时信息**
调用 get_status 获取 GOMAXPROCS、Goroutines 数等
评估 Thanos Query 的资源配置是否充足

## 输出格式

### 查询性能评估
| 维度 | 状态 | 说明 |
|------|------|------|
| Store 组件 | ✅/❌ | x/y 健康 |
| 资源配置 | ✅/⚠️ | GOMAXPROCS=x |

### 查询优化建议
1. 避免查询高基数指标的全量数据
2. 使用 recording rules 预聚合常用查询
3. 合理设置查询超时和最大样本数
4. 确保 Store Gateway 有足够的缓存`}}},
	}
}
