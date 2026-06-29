package thanos

// QueryResult 即时查询结果
type QueryResult struct {
	Status    string     `json:"status"`
	Data      *QueryData `json:"data,omitempty"`
	Error     string     `json:"error,omitempty"`
	ErrorType string     `json:"errorType,omitempty"`
}

// QueryData 查询数据
type QueryData struct {
	ResultType string      `json:"resultType"`
	Result     interface{} `json:"result"`
}

// RangeQueryResult 范围查询结果
type RangeQueryResult struct {
	Status    string          `json:"status"`
	Data      *RangeQueryData `json:"data,omitempty"`
	Error     string          `json:"error,omitempty"`
	ErrorType string          `json:"errorType,omitempty"`
}

// RangeQueryData 范围查询数据
type RangeQueryData struct {
	ResultType string      `json:"resultType"`
	Result     interface{} `json:"result"`
}

// SeriesResult 时间序列查询结果
type SeriesResult struct {
	Status    string                   `json:"status"`
	Data      []map[string]string      `json:"data,omitempty"`
	Error     string                   `json:"error,omitempty"`
	ErrorType string                   `json:"errorType,omitempty"`
}

// LabelNamesResult 标签名查询结果
type LabelNamesResult struct {
	Status    string   `json:"status"`
	Data      []string `json:"data,omitempty"`
	Error     string   `json:"error,omitempty"`
	ErrorType string   `json:"errorType,omitempty"`
}

// LabelValuesResult 标签值查询结果
type LabelValuesResult struct {
	Status    string   `json:"status"`
	Data      []string `json:"data,omitempty"`
	Error     string   `json:"error,omitempty"`
	ErrorType string   `json:"errorType,omitempty"`
}

// TargetsResult 采集目标结果
type TargetsResult struct {
	Status    string       `json:"status"`
	Data      *TargetsData `json:"data,omitempty"`
	Error     string       `json:"error,omitempty"`
	ErrorType string       `json:"errorType,omitempty"`
}

// TargetsData 采集目标数据
type TargetsData struct {
	ActiveTargets  []interface{} `json:"activeTargets"`
	DroppedTargets []interface{} `json:"droppedTargets"`
}

// RulesResult 规则查询结果
type RulesResult struct {
	Status    string     `json:"status"`
	Data      *RulesData `json:"data,omitempty"`
	Error     string     `json:"error,omitempty"`
	ErrorType string     `json:"errorType,omitempty"`
}

// RulesData 规则数据
type RulesData struct {
	Groups []interface{} `json:"groups"`
}

// MetadataResult 指标元数据结果
type MetadataResult struct {
	Status    string        `json:"status"`
	Data      []interface{} `json:"data,omitempty"`
	Error     string        `json:"error,omitempty"`
	ErrorType string        `json:"errorType,omitempty"`
}

// RuntimeInfoResult 运行时信息结果
type RuntimeInfoResult struct {
	Status    string      `json:"status"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	ErrorType string      `json:"errorType,omitempty"`
}

// StoresResult Thanos stores 查询结果
type StoresResult struct {
	Status    string      `json:"status"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	ErrorType string      `json:"errorType,omitempty"`
}

// BuildInfoResult 构建信息结果
type BuildInfoResult struct {
	Status    string      `json:"status"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	ErrorType string      `json:"errorType,omitempty"`
}

// FlagsResult 配置标志结果
type FlagsResult struct {
	Status    string            `json:"status"`
	Data      map[string]string `json:"data,omitempty"`
	Error     string            `json:"error,omitempty"`
	ErrorType string            `json:"errorType,omitempty"`
}

// TSDBStatusResult TSDB 状态结果（Prometheus /api/v1/status/tsdb）
type TSDBStatusResult struct {
	Status    string          `json:"status"`
	Data      *TSDBStatusData `json:"data,omitempty"`
	Error     string          `json:"error,omitempty"`
	ErrorType string          `json:"errorType,omitempty"`
}

// TSDBStatusData TSDB 状态数据
type TSDBStatusData struct {
	HeadStats                *HeadStats        `json:"headStats,omitempty"`
	SeriesCountByMetricName  []TopHeapEntry    `json:"seriesCountByMetricName,omitempty"`
	LabelValueCountByLabelName []TopHeapEntry  `json:"labelValueCountByLabelName,omitempty"`
	MemoryInBytesByLabelName []TopHeapEntry    `json:"memoryInBytesByLabelName,omitempty"`
	SeriesCountByLabelValuePair []TopHeapEntry `json:"seriesCountByLabelValuePair,omitempty"`
}

// HeadStats TSDB Head 块统计
type HeadStats struct {
	NumSeries     int64 `json:"numSeries"`
	NumLabelPairs int64 `json:"numLabelPairs"`
	ChunkCount    int64 `json:"chunkCount"`
	MinTime       int64 `json:"minTime"`
	MaxTime       int64 `json:"maxTime"`
}

// TopHeapEntry TSDB Top N 条目
type TopHeapEntry struct {
	Name  string `json:"name"`
	Value uint64 `json:"value"`
}
