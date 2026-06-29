package tools

import "encoding/json"

// Tool 工具定义
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// GetToolDefinitions 返回工具定义列表
func GetToolDefinitions() []Tool {
	return []Tool{
		// ── 查询 ──
		{
			Name:        "query",
			Description: "执行 PromQL 瞬时查询，用于获取指标实时当前值。必须携带 bi= 或 cluster= 或 kube_cluster_alias= 标签进行租户/集群隔离，确保查询范围精准、高效、无跨集群污染。适用于实时状态核查、指标快照获取。",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "PromQL 查询字符串，必须包含 bi 或 cluster 或 kube_cluster_alias 标签 (如 'up{cluster=\"dev-rke\"}' 或 'metric{kube_cluster_alias=\"dev-rke\"}')"
					}
				},
				"required": ["query"]
			}`),
		},
		{
			Name:        "query_range",
			Description: "执行 PromQL 区间查询，用于获取指标历史趋势、时序变化、周期性波动与性能基线。必须携带 bi= 或 cluster= 或 kube_cluster_alias= 实现多集群隔离，适用于容量分析、故障回溯、趋势预测。",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {
						"type": "string",
						"description": "PromQL 查询字符串，必须包含 bi 或 cluster 或 kube_cluster_alias 标签"
					},
					"start": {
						"type": "number",
						"description": "起始时间 Unix 时间戳（秒）"
					},
					"end": {
						"type": "number",
						"description": "结束时间 Unix 时间戳（秒）"
					},
					"step": {
						"type": "string",
						"description": "查询步长 (如 '15s', '1m', '5m', '1h')"
					}
				},
				"required": ["query", "start", "end", "step"]
			}`),
		},
		// ── 性能/健康 ──
		{
			Name:        "get-targets",
			Description: "获取 Prometheus 采集目标全生命周期状态，包括 active / dropped 目标、健康状态、失败原因与最后采集时间。支持按 scrape_pool/bi/cluster/kube_cluster_alias/job 精准过滤，用于排查采集中断、目标丢失、服务下线、relabel 异常等监控链路根因。【强制过滤】必须提供至少一个过滤参数（scrape_pool / job / bi / cluster / kube_cluster_alias / cluster_alias），避免拉取全量 Targets 导致超时或内存溢出。",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"state": {
						"type": "string",
						"description": "目标状态过滤: active, dropped, any（默认返回全部）",
						"enum": ["active", "dropped", "any"]
					},
					"scrape_pool": {
						"type": "string",
						"description": "可选，按 scrape pool 名称精确过滤（服务端过滤，减少网络传输）"
					},
					"bi": {
						"type": "string",
						"description": "可选，按 bi 标签精确过滤"
					},
					"cluster": {
						"type": "string",
						"description": "可选，按 cluster 标签精确过滤"
					},
					"kube_cluster_alias": {
						"type": "string",
						"description": "可选，按 kube_cluster_alias 标签精确过滤（Kubernetes 集群别名）"
					},
					"cluster_alias": {
						"type": "string",
						"description": "可选，按 cluster_alias 标签精确过滤（人读集群名）"
					},
					"job": {
						"type": "string",
						"description": "可选，按 job 名称关键字过滤（模糊匹配）"
					},
					"limit": {
						"type": "number",
						"description": "可选，限制返回数量，避免大数据量超时"
					}
				}
			}`),
		},
		{
			Name:        "get_rules",
			Description: "获取告警规则与录制规则全量配置，包含规则组、表达式、执行状态、错误信息与标签维度。支持按类型、规则组、文件、集群、租户过滤，用于审计规则有效性、排查告警漏报、录制规则执行失败、高负载规则优化。【强制过滤】必须提供至少一个过滤参数（rule_group / file / group / bi / cluster / kube_cluster_alias / cluster_alias），避免拉取全量 Rules 导致超时或内存溢出。",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"type": {
						"type": "string",
						"description": "规则类型过滤: alert（告警规则）, record（录制规则）",
						"enum": ["alert", "record"]
					},
					"rule_group": {
						"type": "string",
						"description": "可选，按规则组名称精确过滤（服务端过滤，减少网络传输）"
					},
					"file": {
						"type": "string",
						"description": "可选，按规则文件名精确过滤（服务端过滤，减少网络传输）"
					},
					"bi": {
						"type": "string",
						"description": "可选，按 bi 标签精确过滤"
					},
					"cluster": {
						"type": "string",
						"description": "可选，按 cluster 标签精确过滤"
					},
					"kube_cluster_alias": {
						"type": "string",
						"description": "可选，按 kube_cluster_alias 标签精确过滤（Kubernetes 集群别名）"
					},
					"cluster_alias": {
						"type": "string",
						"description": "可选，按 cluster_alias 标签精确过滤（人读集群名）"
					},
					"group": {
						"type": "string",
						"description": "可选，按规则组名称关键字过滤（模糊匹配，客户端过滤）"
					},
					"limit": {
						"type": "number",
						"description": "可选，限制返回规则数量，避免大数据量超时"
					}
				}
			}`),
		},
		{
			Name:        "get-alerts",
			Description: "获取当前活跃告警清单，包含 firing/pending 状态、告警来源、标签、触发时间与持续时长。支持按级别过滤，提供告警统计摘要，用于实时故障巡检、告警风暴识别、SLA 违规判定。",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"severity": {
						"type": "string",
						"description": "可选，按告警级别过滤 (如 critical, warning, info)"
					}
				}
			}`),
		},
		// ── 运维辅助 ──
		{
			Name:        "get-cluster-info",
			Description: "获取 Prometheus/Thanos 集群核心运行时信息，包括存储策略、数据保留周期、WAL 状态、资源限制、启动配置。用于集群健康巡检、容量规划、配置审计与异常排障。",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		},
		{
			Name:        "get_status",
			Description: "获取系统运行态与构建信息，含版本、构建信息、启动时间、Goroutines、GC 配置、资源使用概况。用于版本一致性校验、环境诊断、性能瓶颈定位。",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		},
		{
			Name:        "get_stores",
			Description: "获取 Thanos StoreAPI 全量注册节点，包括 sidecar、store gateway、ruler、receive 等组件状态、时间覆盖范围、外部标签与最后健康状态。自动输出集群-租户-别名映射关系，用于排查数据不可查、存储节点掉线、分片异常、集群标签错乱等架构级问题。",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"type": {
						"type": "string",
						"description": "可选，按 store 类型过滤 (如 sidecar, store, rule, receive)"
					}
				}
			}`),
		},
		// ── 高基数分析 ──
		{
			Name:        "get_cardinality_analysis",
			Description: "Prometheus 高基数智能深度分析，基于 TSDB Status 实时采集 TopN 高基数指标、标签爆炸、标签值分布、内存占用与 job 级序列数。用于定位时序膨胀、优化采集策略、降低查询负载、缓解 OOM、指导分片与存储架构优化。",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"prometheus_endpoint": {
						"type": "string",
						"description": "必填，目标 Prometheus 实例地址（如 http://prometheus:9090）"
					},
					"bi": {
						"type": "string",
						"description": "BI 标签值，用于限定分析范围（与 cluster、kube_cluster_alias 三选一必填）"
					},
					"cluster": {
						"type": "string",
						"description": "集群标签值，用于限定分析范围（与 bi、kube_cluster_alias 三选一必填）"
					},
					"kube_cluster_alias": {
						"type": "string",
						"description": "Kubernetes 集群别名，用于限定分析范围（与 bi、cluster 三选一必填）"
					},
					"limit": {
						"type": "number",
						"description": "可选，Top N 返回数量，默认 20"
					}
				},
				"required": ["prometheus_endpoint"]
			}`),
		},
	}
}