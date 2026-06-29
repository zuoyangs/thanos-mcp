package transport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"thanos-mcp/config"
	"thanos-mcp/tools"
)

// HTTPHandler HTTP 处理器
type HTTPHandler struct {
	server  *tools.MCPServer
	handler *tools.Handler
	logger  Logger
}

// NewHTTPHandler 创建 HTTP 处理器
func NewHTTPHandler(server *tools.MCPServer, logger Logger) *HTTPHandler {
	return &HTTPHandler{
		server:  server,
		handler: &tools.Handler{Server: server},
		logger:  logger,
	}
}

// Logger 日志接口
type Logger interface {
	Infof(format string, args ...interface{})
	Debugf(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Accessf(format string, args ...interface{})
}

// ServeHTTP 实现 http.Handler 接口
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	clientIP := GetClientIP(r)
	authHeader := r.Header.Get("Authorization")
	username := h.server.Auth.ExtractUsername(authHeader)
	startTime := time.Now()

	if r.Method != http.MethodOptions && h.server.Auth.Enabled {
		// 认证下沉到 processHTTPRequest 层按 JSON-RPC method 判断，
		// 导入 tools 无需认证，仅 chat 交互需要认证
		h.logger.Accessf("REQUEST | IP: %-15s | User: %-15s | Method: %-8s | Path: %s",
			clientIP, username, r.Method, r.URL.Path)
	}

	switch r.Method {
	case http.MethodOptions:
		h.handleCORS(w)
	case http.MethodPost:
		h.handlePost(w, r, clientIP, username, startTime)
	case http.MethodGet:
		h.handleGet(w, r, clientIP, username)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func (h *HTTPHandler) handleCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.WriteHeader(http.StatusOK)
}

func (h *HTTPHandler) handlePost(w http.ResponseWriter, r *http.Request, clientIP, username string, startTime time.Time) {
	switch r.URL.Path {
	case "/initialize":
		h.logger.Infof("初始化请求 | IP: %s | User: %s | Path: %s", clientIP, username, r.URL.Path)
		h.handleInitialize(w)
	case "/tools/call":
		h.handleCallTool(w, r, clientIP, username)
	case "/mcp", "/":
		h.handleMCPRequest(w, r, clientIP, username, startTime)
	default:
		h.logger.Accessf("请求 404 | IP: %-15s | User: %-15s | Method: %-8s | Path: %s",
			clientIP, username, r.Method, r.URL.Path)
		http.Error(w, "Not Found", http.StatusNotFound)
	}
}

func (h *HTTPHandler) handleMCPRequest(w http.ResponseWriter, r *http.Request, clientIP, username string, startTime time.Time) {
	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Errorf("JSON解析失败 | IP: %s | Error: %v", clientIP, err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	resp := h.processHTTPRequest(&req, clientIP, username, startTime)
	resp.ID = req.RequestID()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Errorf("响应编码失败 | IP: %s | Error: %v", clientIP, err)
	}

	h.logger.Infof("MCP 请求处理完成 | IP: %s | User: %s | Duration: %v", clientIP, username, time.Since(startTime))
}

func (h *HTTPHandler) processHTTPRequest(req *JSONRPCRequest, clientIP, username string, startTime time.Time) JSONRPCResponse {
	// 导入 tools 相关的方法无需认证，仅 chat 交互需要认证
	switch req.Method {
	case "initialize", "ping", "tools/list", "prompts/list", "resources/list":
	default:
		if h.server.Auth.Enabled {
			authHeader := ""
			if headers, ok := req.Params["_auth_header"].(string); ok {
				authHeader = headers
			}
			if meta, ok := req.Params["_meta"].(map[string]interface{}); ok {
				if auth, ok := meta["authorization"].(string); ok {
					authHeader = auth
				}
			}
			if !h.server.Auth.ValidateAuth(authHeader) {
				h.logger.Warnf("认证失败 | IP: %s | Method: %s", clientIP, req.Method)
				return JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.RequestID(),
					Error: &JSONError{
						Code:    -32603,
						Message: "Unauthorized",
					},
				}
			}
		}
	}

	switch req.Method {
	case "tools/list":
		h.logger.Infof("tools/list | IP: %s | User: %s | Duration: %v", clientIP, username, time.Since(startTime))
	case "tools/call":
		name, _ := req.Params["name"].(string)
		args, _ := req.Params["arguments"].(map[string]interface{})
		authHeader := h.getAuthHeaderFromRequest(req)
		userInfo := username
		if username == "unknown" {
			userInfo = fmt.Sprintf("unknown (token: %s)", config.MaskBearerToken(config.ExtractBearerToken(authHeader)))
		}
		h.logger.Accessf("MCP REQUEST | IP: %-15s | User: %-50s | Tool: %s | Args: %v | Duration: %v",
			clientIP, userInfo, name, args, time.Since(startTime))
	case "mcp:restart-server":
		h.logger.Infof("mcp:restart-server | IP: %s | User: %s | Duration: %v", clientIP, username, time.Since(startTime))
	case "mcp:shutdown":
		h.logger.Infof("mcp:shutdown | IP: %s | User: %s | Duration: %v", clientIP, username, time.Since(startTime))
	}

	return ProcessToolsRequest(h.handler, req)
}

// getAuthHeaderFromRequest 从请求参数中提取 auth header
func (h *HTTPHandler) getAuthHeaderFromRequest(req *JSONRPCRequest) string {
	if req.Params == nil {
		return ""
	}
	if header, ok := req.Params["_auth_header"].(string); ok && header != "" {
		return header
	}
	if meta, ok := req.Params["_meta"].(map[string]interface{}); ok {
		if auth, ok := meta["authorization"].(string); ok && auth != "" {
			return auth
		}
	}
	return ""
}

func (h *HTTPHandler) handleGet(w http.ResponseWriter, r *http.Request, clientIP, username string) {
	if r.URL.Path == "/tools/list" {
		h.logger.Infof("工具列表请求 | IP: %s | User: %s", clientIP, username)
		h.handleListTools(w)
	} else {
		h.logger.Accessf("请求 404 | IP: %-15s | User: %-15s | Method: %-8s | Path: %s",
			clientIP, username, r.Method, r.URL.Path)
		http.Error(w, "Not Found", http.StatusNotFound)
	}
}

func (h *HTTPHandler) handleInitialize(w http.ResponseWriter) {
	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]bool{
				"list": true,
				"call": true,
			},
			"prompts": map[string]bool{
				"list": true,
			},
			"resources": map[string]bool{
				"list":      true,
				"subscribe": false,
			},
			"sampling": map[string]bool{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "thanos-mcp",
			"version": "1.0.0",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      0,
		Result:  result,
	})
}

func (h *HTTPHandler) handleListTools(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      0,
		Result:  map[string]interface{}{"tools": tools.GetToolDefinitions()},
	})
}

func (h *HTTPHandler) handleCallTool(w http.ResponseWriter, r *http.Request, clientIP, username string) {
	var request map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.logger.Errorf("JSON解析失败 | IP: %s | User: %s | Error: %v", clientIP, username, err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	id, _ := request["id"].(float64)
	params, _ := request["params"].(map[string]interface{})

	name, _ := params["name"].(string)
	arguments, _ := params["arguments"].(map[string]interface{})

	queryStr := ""
	if args, ok := arguments["query"].(string); ok {
		queryStr = args
	}
	h.logger.Accessf("QUERY REQUEST | IP: %-15s | User: %-15s | Tool: %-12s | Query: %s | Args: %v",
		clientIP, username, name, queryStr, arguments)
	h.logger.Infof("查询请求 | IP: %s | User: %s | Tool: %s | Query: %s", clientIP, username, name, queryStr)

	queryStartTime := time.Now()
	result, err := h.handler.HandleCallTool(r.Context(), name, arguments)
	queryDuration := time.Since(queryStartTime)

	resultPreview := ""
	resultSuccess := true
	if err != nil {
		resultPreview = fmt.Sprintf("ERROR: %v", err)
		resultSuccess = false
	} else if resultMap, ok := result.(map[string]interface{}); ok {
		if content, ok := resultMap["content"].([]interface{}); ok && len(content) > 0 {
			if textMap, ok := content[0].(map[string]interface{}); ok {
				if text, ok := textMap["text"].(string); ok {
					if len(text) > 200 {
						resultPreview = text[:200] + "... [truncated]"
					} else {
						resultPreview = text
					}
				}
			}
		}
		if isError, ok := resultMap["isError"].(bool); ok && isError {
			resultSuccess = false
		}
	}

	h.logger.Accessf("QUERY RESULT | IP: %-15s | User: %-15s | Tool: %-12s | Status: %-7s | Duration: %-10v | Result: %s",
		clientIP, username, name, map[bool]string{true: "SUCCESS", false: "FAILED"}[resultSuccess], queryDuration, resultPreview)

	if resultSuccess {
		h.logger.Infof("查询成功 | IP: %s | User: %s | Tool: %s | Duration: %v", clientIP, username, name, queryDuration)
	} else {
		h.logger.Errorf("查询失败 | IP: %s | User: %s | Tool: %s | Error: %s | Duration: %v", clientIP, username, name, resultPreview, queryDuration)
	}

	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	if err != nil {
		response.Error = &JSONError{
			Code:    -32603,
			Message: err.Error(),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Errorf("响应编码失败 | IP: %s | Error: %v", clientIP, err)
	}
}

// RunHTTPTransport 启动 HTTP 传输模式
func RunHTTPTransport(server *tools.MCPServer, port int, logger Logger) {
	logger.Infof("========================================")
	logger.Infof("Thanos MCP Server 启动 (HTTP mode)")
	logger.Infof("Thanos endpoint: %s", server.ThanosClient.Endpoint)
	if server.Auth.Enabled {
		logger.Infof("认证: 已启用 | 用户数: %d", len(server.Auth.Users))
	} else {
		logger.Infof("认证: 已禁用")
	}
	logger.Infof("========================================")

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("Server listening on %s\n", addr)
	handler := NewHTTPHandler(server, logger)
	if err := http.ListenAndServe(addr, handler); err != nil {
		logger.Errorf("服务器错误: %v", err)
		fmt.Printf("Server error: %v\n", err)
	}
}
