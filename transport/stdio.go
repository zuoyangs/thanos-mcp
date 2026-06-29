package transport

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"thanos-mcp/prompts"
	"thanos-mcp/tools"
)

// StdioTransport STDIO 传输
type StdioTransport struct {
	server  *tools.MCPServer
	handler *tools.Handler
	logger  Logger
}

// NewStdioTransport 创建 STDIO 传输
func NewStdioTransport(server *tools.MCPServer, logger Logger) *StdioTransport {
	return &StdioTransport{
		server:  server,
		handler: &tools.Handler{Server: server},
		logger:  logger,
	}
}

// Run 启动 STDIO 传输
func (t *StdioTransport) Run() {
	t.logger.Infof("========================================")
	t.logger.Infof("Thanos MCP Server 启动 (stdio mode)")
	t.logger.Infof("Thanos endpoint: %s", t.server.ThanosClient.Endpoint)
	if t.server.Auth.Enabled {
		t.logger.Infof("认证: 已启用 | 用户数: %d", len(t.server.Auth.Users))
		for _, u := range t.server.Auth.Users {
			t.logger.Debugf("已配置用户: %s", u.Username)
		}
	} else {
		t.logger.Infof("认证: 已禁用")
	}
	t.logger.Infof("========================================")

	log.SetOutput(os.Stderr)

	// 使用带缓冲的 writer 确保响应立即发送
	bufWriter := bufio.NewWriter(os.Stdout)
	defer bufWriter.Flush()

	dec := json.NewDecoder(os.Stdin)
	enc := json.NewEncoder(bufWriter)
	enc.SetEscapeHTML(false)

	for {
		var req JSONRPCRequest
		if err := dec.Decode(&req); err != nil {
			if err == io.EOF {
				t.logger.Infof("收到 EOF，客户端已断开连接")
				break
			}
			t.logger.Errorf("stdio decode error: %v", err)
			continue
		}

		req.Method = strings.TrimSpace(req.Method)

		// 始终记录收到的请求（因为日志级别可调，这里打印关键信息）
		reqID := req.RequestID()
		t.logger.Debugf("收到请求 | Method: %s | ID: %v | HasParams: %v", req.Method, reqID, len(req.Params) > 0)
		if len(req.Params) > 0 {
			paramsJSON, _ := json.Marshal(req.Params)
			t.logger.Debugf("请求参数(raw): %s", string(paramsJSON))
		}

		if req.IsNotification() || strings.HasPrefix(req.Method, "notifications/") {
			t.logger.Debugf("跳过通知请求: %s", req.Method)
			continue
		}

		t.logger.Infof(">>> 处理请求 | Method: %s | ID: %v", req.Method, reqID)

		// 保护处理逻辑，防止 panic 导致进程退出
		var resp JSONRPCResponse
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.logger.Errorf("处理请求时发生 panic: %v", r)
					resp = JSONRPCResponse{
						JSONRPC: "2.0",
						ID:      reqID,
						Error: &JSONError{
							Code:    -32603,
							Message: fmt.Sprintf("Internal error: %v", r),
						},
					}
				}
			}()
			resp = t.processRequest(&req)
		}()

		resp.JSONRPC = "2.0"
		resp.ID = reqID

		if resp.Error != nil {
			t.logger.Errorf("<<< 请求返回错误 | Method: %s | Code: %d | Message: %s", req.Method, resp.Error.Code, resp.Error.Message)
		} else {
			resultPreview := ""
			if resp.Result != nil {
				if m, ok := resp.Result.(map[string]interface{}); ok {
					if tools, ok := m["tools"].([]interface{}); ok {
						resultPreview = fmt.Sprintf("%d tools", len(tools))
					} else {
						resultPreview = fmt.Sprintf("%+v", m)
					}
				}
			}
			t.logger.Infof("<<< 请求成功 | Method: %s | Result: %s", req.Method, resultPreview)
		}

		if err := enc.Encode(resp); err != nil {
			t.logger.Errorf("stdio encode error: %v", err)
			break
		}
		if err := bufWriter.Flush(); err != nil {
			t.logger.Errorf("刷新输出缓冲区失败: %v", err)
			break
		}
	}
	t.logger.Infof("STDIO 传输循环结束")
}

func (t *StdioTransport) processRequest(req *JSONRPCRequest) JSONRPCResponse {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
	}

	// 检查是否需要认证的方法：导入 tools 相关的方法无需认证
	needsAuth := true
	switch req.Method {
	case "initialize", "ping", "tools/list", "prompts/list", "resources/list":
		needsAuth = false
	}
	if needsAuth && t.server.Auth.Enabled {
		t.logger.Infof("认证检查 | Method: %s | 认证已启用", req.Method)

		authHeader := ""
		if headers, ok := req.Params["_auth_header"].(string); ok {
			authHeader = headers
			t.logger.Infof("从 _auth_header 获取认证信息: %s", authHeader)
		} else {
			t.logger.Infof("未找到 _auth_header 参数，检查其他认证方式")
			// 尝试从 _meta 或其他字段获取认证
			if meta, ok := req.Params["_meta"].(map[string]interface{}); ok {
				if auth, ok := meta["authorization"].(string); ok {
					authHeader = auth
					t.logger.Infof("从 _meta.authorization 获取: %s", authHeader)
				}
			}
			// 尝试从 _extraHeaders 获取
			if extra, ok := req.Params["_extraHeaders"].(map[string]interface{}); ok {
				if auth, ok := extra["Authorization"].(string); ok {
					authHeader = auth
					t.logger.Infof("从 _extraHeaders.Authorization 获取: %s", authHeader)
				}
			}
		}

		if authHeader == "" {
			t.logger.Warnf("认证失败 | Method: %s | 原因: 未提供认证信息", req.Method)
			t.logger.Warnf("请求参数中的所有 key: %v", func() []string {
				keys := make([]string, 0, len(req.Params))
				for k := range req.Params {
					keys = append(keys, k)
				}
				return keys
			}())
			response.Error = &JSONError{
				Code:    -32603,
				Message: "Unauthorized: Missing credentials (HTTP 401)",
			}
			return response
		}

		if !t.server.Auth.ValidateAuth(authHeader) {
			t.logger.Warnf("认证失败 | Method: %s | AuthHeader: %s | 原因: 认证信息无效", req.Method, authHeader)
			response.Error = &JSONError{
				Code:    -32603,
				Message: "Unauthorized: Invalid credentials (HTTP 401)",
			}
			return response
		}
		t.logger.Infof("认证成功 | Method: %s", req.Method)
	}

	switch req.Method {
	case "initialize":
		t.logger.Infof("处理 initialize 请求")
		pv := "2024-11-05"
		if req.Params != nil {
			if v, ok := req.Params["protocolVersion"].(string); ok && v != "" {
				pv = v
			}
			if clientInfo, ok := req.Params["clientInfo"].(map[string]interface{}); ok {
				t.logger.Debugf("客户端信息: %+v", clientInfo)
			}
		}
		response.Result = map[string]interface{}{
			"protocolVersion": pv,
			"capabilities": map[string]interface{}{
				"tools": map[string]bool{
					"list":   true,
					"call":   true,
				},
				"prompts": map[string]bool{
					"list": true,
				},
				"resources": map[string]bool{
					"list":   true,
					"subscribe": false,
				},
				"sampling": map[string]bool{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "thanos-mcp",
				"version": "1.0.0",
			},
		}
		t.logger.Infof("initialize 成功 | protocolVersion: %s", pv)

	case "ping":
		t.logger.Debugf("处理 ping 请求")
		response.Result = map[string]interface{}{}

	case "tools/list":
		t.logger.Infof("处理 tools/list 请求")
		toolsList := tools.GetToolDefinitions()
		t.logger.Infof("返回 %d 个工具定义", len(toolsList))
		for _, tool := range toolsList {
			t.logger.Debugf("工具: %s - %s", tool.Name, tool.Description)
		}
		response.Result = map[string]interface{}{
			"tools": toolsList,
		}

	case "tools/call":
		t.logger.Infof("处理 tools/call 请求")
		params := req.Params
		if params == nil {
			params = map[string]any{}
		}
		name, _ := params["name"].(string)
		arguments, _ := params["arguments"].(map[string]interface{})

		t.logger.Infof("工具调用 | Name: %s | Arguments: %+v", name, arguments)

		result, err := t.handler.HandleCallTool(DefaultContext, name, arguments)
		if err != nil {
			t.logger.Errorf("工具调用失败 | Name: %s | Error: %v", name, err)
			response.Error = &JSONError{
				Code:    -32603,
				Message: err.Error(),
			}
		} else {
			t.logger.Infof("工具调用成功 | Name: %s", name)
			response.Result = result
		}

	// Cherry Studio / MCP 扩展方法
	case "mcp:restart-server":
		t.logger.Infof("收到服务器重启请求 (Cherry Studio)")
		t.logger.Infof("服务器重启 | 注意: STDIO 模式下无法自动重启，请手动重启服务")
		response.Result = map[string]interface{}{
			"success": true,
			"message": "Server restart requested. Please manually restart the MCP server.",
		}

	case "mcp:shutdown":
		t.logger.Infof("收到服务器关闭请求")
		response.Result = map[string]interface{}{
			"success": true,
			"message": "Server shutdown requested",
		}

	case "mcp:server-info":
		t.logger.Debugf("收到服务器信息请求")
		response.Result = map[string]interface{}{
			"name":           "thanos-mcp",
			"version":        "1.0.0",
			"transport":      t.server.Transport,
			"thanosEndpoint": t.server.ThanosClient.Endpoint,
			"authEnabled":    t.server.Auth.Enabled,
		}

	case "sampling/createMessage":
		t.logger.Debugf("收到采样请求")
		response.Result = map[string]interface{}{
			"model":      "thanos-mcp",
			"stopReason": "end_turn",
			"content": []map[string]string{
				{"type": "text", "text": "This server does not support AI sampling."},
			},
		}

	case "prompts/list":
		t.logger.Debugf("收到 prompts/list 请求")
		response.Result = map[string]interface{}{
			"prompts": prompts.GetAllPrompts(),
		}

	case "prompts/get":
		t.logger.Debugf("收到 prompts/get 请求")
		var params struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments"`
		}
		if req.Params != nil {
			raw, _ := json.Marshal(req.Params)
			json.Unmarshal(raw, &params)
		}
		result := prompts.HandleGetPrompt(params.Name, params.Arguments)
		response.Result = result

	case "resources/list":
		t.logger.Debugf("收到 resources/list 请求")
		response.Result = map[string]interface{}{
			"resources": []interface{}{},
		}

	case "roots/list":
		t.logger.Debugf("收到 roots/list 请求")
		response.Result = map[string]interface{}{
			"roots": []interface{}{},
		}

	case "roots/add":
		t.logger.Debugf("收到 roots/add 请求")
		response.Result = map[string]interface{}{
			"success": true,
		}

	default:
		t.logger.Warnf("未知方法: %s", req.Method)
		response.Error = &JSONError{
			Code:    -32601,
			Message: fmt.Sprintf("Method not found: %s", req.Method),
		}
	}

	return response
}
