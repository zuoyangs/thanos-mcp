package thanos

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client Thanos 客户端结构体
type Client struct {
	Endpoint string        // 服务地址
	Timeout  time.Duration // 请求超时时间
	Client   *http.Client   // HTTP 客户端
}

// NewClient 创建一个新的 Thanos 客户端
func NewClient(endpoint string, timeout time.Duration) *Client {
	return &Client{
		Endpoint: endpoint,
		Timeout:  timeout,
		Client: &http.Client{
			Timeout: timeout,
		},
	}
}

// doGet 通用 GET 请求方法
func (c *Client) doGet(ctx context.Context, urlStr string) ([]byte, error) {
	// 创建带上下文的 GET 请求
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "identity")

	// 执行 HTTP 请求
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("执行请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	// 校验响应状态码
	if resp.StatusCode != http.StatusOK {
		preview := string(body)
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		return nil, fmt.Errorf("Thanos 返回 HTTP 状态码 %d: %s", resp.StatusCode, preview)
	}
	return body, nil
}

// doPost 通用 POST 请求（表单编码格式）
func (c *Client) doPost(ctx context.Context, urlStr string, form url.Values) ([]byte, error) {
	// 创建带上下文的 POST 请求
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// 执行 HTTP 请求
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("执行请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	// 校验响应状态码
	if resp.StatusCode != http.StatusOK {
		preview := string(body)
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		return nil, fmt.Errorf("Thanos 返回 HTTP 状态码 %d: %s", resp.StatusCode, preview)
	}
	return body, nil
}

// checkEndpoint 校验服务地址是否合法
func (c *Client) checkEndpoint() error {
	if c.Endpoint == "" {
		return fmt.Errorf("Thanos 服务地址未配置")
	}
	if !strings.HasPrefix(c.Endpoint, "http://") && !strings.HasPrefix(c.Endpoint, "https://") {
		return fmt.Errorf("Thanos 服务地址必须以 http:// 或 https:// 开头，当前地址: %s", c.Endpoint)
	}
	return nil
}

// Query 对 Thanos/Prometheus 执行瞬时查询
func (c *Client) Query(ctx context.Context, query string) (*QueryResult, error) {
	// 校验服务地址
	if err := c.checkEndpoint(); err != nil {
		return nil, err
	}

	// 构造查询 URL
	urlStr := fmt.Sprintf("%s/api/v1/query?query=%s", c.Endpoint, url.QueryEscape(query))
	// 发送请求
	body, err := c.doGet(ctx, urlStr)
	if err != nil {
		return nil, err
	}

	// 解析 JSON 响应
	var result QueryResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return &result, nil
}

// RangeQuery 对 Thanos/Prometheus 执行区间查询
func (c *Client) RangeQuery(ctx context.Context, query string, start, end int64, step string) (*RangeQueryResult, error) {
	// 校验服务地址
	if err := c.checkEndpoint(); err != nil {
		return nil, err
	}

	// 构造区间查询 URL
	urlStr := fmt.Sprintf("%s/api/v1/query_range?query=%s&start=%d&end=%d&step=%s",
		c.Endpoint, url.QueryEscape(query), start, end, url.QueryEscape(step))
	// 发送请求
	body, err := c.doGet(ctx, urlStr)
	if err != nil {
		return nil, err
	}

	// 解析 JSON 响应
	var result RangeQueryResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return &result, nil
}

// Series 查询匹配标签选择器的时间序列
func (c *Client) Series(ctx context.Context, matchers []string, start, end int64) (*SeriesResult, error) {
	// 校验服务地址
	if err := c.checkEndpoint(); err != nil {
		return nil, err
	}

	// 构造请求参数
	form := url.Values{}
	for _, m := range matchers {
		form.Add("match[]", m)
	}
	if start > 0 {
		form.Set("start", fmt.Sprintf("%d", start))
	}
	if end > 0 {
		form.Set("end", fmt.Sprintf("%d", end))
	}

	// 发送 POST 请求
	urlStr := fmt.Sprintf("%s/api/v1/series", c.Endpoint)
	body, err := c.doPost(ctx, urlStr, form)
	if err != nil {
		return nil, err
	}

	// 解析 JSON 响应
	var result SeriesResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return &result, nil
}

// LabelNames 获取所有标签名称
func (c *Client) LabelNames(ctx context.Context, matchers []string, start, end int64) (*LabelNamesResult, error) {
	// 校验服务地址
	if err := c.checkEndpoint(); err != nil {
		return nil, err
	}

	// 构造请求参数
	params := url.Values{}
	for _, m := range matchers {
		params.Add("match[]", m)
	}
	if start > 0 {
		params.Set("start", fmt.Sprintf("%d", start))
	}
	if end > 0 {
		params.Set("end", fmt.Sprintf("%d", end))
	}

	// 发送 GET 请求
	urlStr := fmt.Sprintf("%s/api/v1/labels?%s", c.Endpoint, params.Encode())
	body, err := c.doGet(ctx, urlStr)
	if err != nil {
		return nil, err
	}

	// 解析 JSON 响应
	var result LabelNamesResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return &result, nil
}

// LabelValues 获取指定标签名称的取值
func (c *Client) LabelValues(ctx context.Context, labelName string, matchers []string, start, end int64) (*LabelValuesResult, error) {
	// 校验服务地址
	if err := c.checkEndpoint(); err != nil {
		return nil, err
	}

	// 构造请求参数
	params := url.Values{}
	for _, m := range matchers {
		params.Add("match[]", m)
	}
	if start > 0 {
		params.Set("start", fmt.Sprintf("%d", start))
	}
	if end > 0 {
		params.Set("end", fmt.Sprintf("%d", end))
	}

	// 发送 GET 请求
	urlStr := fmt.Sprintf("%s/api/v1/label/%s/values?%s", c.Endpoint, url.PathEscape(labelName), params.Encode())
	body, err := c.doGet(ctx, urlStr)
	if err != nil {
		return nil, err
	}

	// 解析 JSON 响应
	var result LabelValuesResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return &result, nil
}

// Targets 获取采集目标及其状态
// state: 状态过滤
// scrapePool: 采集池过滤，仅返回指定采集池的目标
func (c *Client) Targets(ctx context.Context, state string, scrapePool string) (*TargetsResult, error) {
	// 校验服务地址
	if err := c.checkEndpoint(); err != nil {
		return nil, err
	}

	// 构造请求参数
	params := url.Values{}
	if state != "" {
		params.Set("state", state)
	}
	if scrapePool != "" {
		params.Set("scrapePool", scrapePool)
	}

	// 构造 URL 并发送请求
	urlStr := fmt.Sprintf("%s/api/v1/targets", c.Endpoint)
	if encoded := params.Encode(); encoded != "" {
		urlStr += "?" + encoded
	}
	body, err := c.doGet(ctx, urlStr)
	if err != nil {
		return nil, err
	}

	// 解析 JSON 响应
	var result TargetsResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return &result, nil
}

// Rules 获取告警规则和记录规则
// ruleType: 规则类型，alert 或 record
// ruleGroups: 规则组名称列表
// files: 规则文件列表
func (c *Client) Rules(ctx context.Context, ruleType string, ruleGroups []string, files []string) (*RulesResult, error) {
	// 校验服务地址
	if err := c.checkEndpoint(); err != nil {
		return nil, err
	}

	// 构造请求参数
	params := url.Values{}
	if ruleType != "" {
		params.Set("type", ruleType)
	}
	for _, g := range ruleGroups {
		if g != "" {
			params.Add("rule_group[]", g)
		}
	}
	for _, f := range files {
		if f != "" {
			params.Add("file[]", f)
		}
	}

	// 构造 URL 并发送请求
	urlStr := fmt.Sprintf("%s/api/v1/rules", c.Endpoint)
	if encoded := params.Encode(); encoded != "" {
		urlStr += "?" + encoded
	}
	body, err := c.doGet(ctx, urlStr)
	if err != nil {
		return nil, err
	}

	// 解析 JSON 响应
	var result RulesResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return &result, nil
}

// TargetMetadata 获取采集目标的指标元数据
func (c *Client) TargetMetadata(ctx context.Context, matchTarget, metric string, limit int) (*MetadataResult, error) {
	// 校验服务地址
	if err := c.checkEndpoint(); err != nil {
		return nil, err
	}

	// 构造请求参数
	params := url.Values{}
	if matchTarget != "" {
		params.Set("match_target", matchTarget)
	}
	if metric != "" {
		params.Set("metric", metric)
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}

	// 发送 GET 请求
	urlStr := fmt.Sprintf("%s/api/v1/targets/metadata?%s", c.Endpoint, params.Encode())
	body, err := c.doGet(ctx, urlStr)
	if err != nil {
		return nil, err
	}

	// 解析 JSON 响应
	var result MetadataResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return &result, nil
}

// RuntimeInfo 获取运行时信息
func (c *Client) RuntimeInfo(ctx context.Context) (*RuntimeInfoResult, error) {
	// 校验服务地址
	if err := c.checkEndpoint(); err != nil {
		return nil, err
	}

	// 发送请求
	urlStr := fmt.Sprintf("%s/api/v1/status/runtimeinfo", c.Endpoint)
	body, err := c.doGet(ctx, urlStr)
	if err != nil {
		return nil, err
	}

	// 解析 JSON 响应
	var result RuntimeInfoResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return &result, nil
}

// Stores 获取 Thanos 存储 API 端点信息
func (c *Client) Stores(ctx context.Context) (*StoresResult, error) {
	// 校验服务地址
	if err := c.checkEndpoint(); err != nil {
		return nil, err
	}

	// 发送请求
	urlStr := fmt.Sprintf("%s/api/v1/stores", c.Endpoint)
	body, err := c.doGet(ctx, urlStr)
	if err != nil {
		return nil, err
	}

	// 解析 JSON 响应
	var result StoresResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return &result, nil
}

// BuildInfo 获取构建信息
func (c *Client) BuildInfo(ctx context.Context) (*BuildInfoResult, error) {
	// 校验服务地址
	if err := c.checkEndpoint(); err != nil {
		return nil, err
	}

	// 发送请求
	urlStr := fmt.Sprintf("%s/api/v1/status/buildinfo", c.Endpoint)
	body, err := c.doGet(ctx, urlStr)
	if err != nil {
		return nil, err
	}

	// 解析 JSON 响应
	var result BuildInfoResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return &result, nil
}

// TSDBStatus 获取 TSDB 头部状态（含基数TopN指标）
// selector: 外部标签过滤
// limit: 返回数量限制
func (c *Client) TSDBStatus(ctx context.Context, selector string, limit int) (*TSDBStatusResult, error) {
	// 校验服务地址
	if err := c.checkEndpoint(); err != nil {
		return nil, err
	}

	// 构造请求参数
	params := url.Values{}
	if selector != "" {
		params.Set("match[]", selector)
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}

	// 构造 URL 并发送请求
	urlStr := fmt.Sprintf("%s/api/v1/status/tsdb", c.Endpoint)
	if len(params) > 0 {
		urlStr += "?" + params.Encode()
	}
	body, err := c.doGet(ctx, urlStr)
	if err != nil {
		return nil, err
	}

	// 解析 JSON 响应
	var result TSDBStatusResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析 TSDB 状态响应失败: %w", err)
	}
	return &result, nil
}

// TSDBStatusFromPrometheus 直接请求 Prometheus 实例的 TSDB 状态
// Thanos Query 不提供该 API，必须直接查询 Prometheus
func (c *Client) TSDBStatusFromPrometheus(ctx context.Context, prometheusEndpoint string, limit int) (*TSDBStatusResult, error) {
	// 校验 Prometheus 地址
	if prometheusEndpoint == "" {
		return nil, fmt.Errorf("必须提供 Prometheus 地址：Thanos Query 不暴露 /api/v1/status/tsdb，需直接查询 Prometheus")
	}
	if !strings.HasPrefix(prometheusEndpoint, "http://") && !strings.HasPrefix(prometheusEndpoint, "https://") {
		return nil, fmt.Errorf("Prometheus 地址必须以 http:// 或 https:// 开头，当前地址: %s", prometheusEndpoint)
	}

	// 构造请求参数
	params := url.Values{}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}

	// 构造 URL 并发送请求
	urlStr := fmt.Sprintf("%s/api/v1/status/tsdb", strings.TrimRight(prometheusEndpoint, "/"))
	if len(params) > 0 {
		urlStr += "?" + params.Encode()
	}
	body, err := c.doGet(ctx, urlStr)
	if err != nil {
		return nil, fmt.Errorf("查询 Prometheus TSDB 状态失败: %w", err)
	}

	// 解析 JSON 响应
	var result TSDBStatusResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析 Prometheus TSDB 状态响应失败: %w", err)
	}
	return &result, nil
}

// Flags 获取服务配置参数
func (c *Client) Flags(ctx context.Context) (*FlagsResult, error) {
	// 校验服务地址
	if err := c.checkEndpoint(); err != nil {
		return nil, err
	}

	// 发送请求
	urlStr := fmt.Sprintf("%s/api/v1/status/flags", c.Endpoint)
	body, err := c.doGet(ctx, urlStr)
	if err != nil {
		return nil, err
	}

	// 解析 JSON 响应
	var result FlagsResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return &result, nil
}