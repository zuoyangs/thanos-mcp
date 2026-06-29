package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"thanos-mcp/config"
	"thanos-mcp/tools"
	"thanos-mcp/transport"
	"thanos-mcp/utils"
)

func main() {
	configFile := flag.String("config", "etc/config.yaml", "Path to config file")
	flag.Parse()

	logger := initLogger()

	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		logger.Errorf("配置加载失败: %v", err)
		log.Fatalf("配置加载失败: %v", err)
	}

	logger.Infof("========== 启动参数 ==========")
	logger.Infof("Thanos Endpoint (来自配置): %s", cfg.ThanosEndpoint)
	logger.Infof("Thanos Timeout: %s", cfg.ThanosTimeout)
	logger.Infof("Transport: %s", cfg.Transport)
	logger.Infof("Port: %d", cfg.Port)
	logger.Infof("===============================")

	server := tools.NewMCPServer(cfg.ThanosEndpoint, cfg.ThanosTimeout, cfg.Transport, cfg.Auth)

	switch cfg.Transport {
	case config.TransportStdio:
		transport.NewStdioTransport(server, logger).Run()

	case config.TransportStreamableHttp:
		transport.RunStreamableHttpTransport(server, cfg.Port, logger)

	default:
		transport.RunHTTPTransport(server, cfg.Port, logger)
	}
}

// defaultLogDir returns ./logs when the process cwd is writable; otherwise a
// user-writable directory (e.g. under %LocalAppData% on Windows). MCP hosts
// often run the binary with a cwd that cannot mkdir "logs".
func defaultLogDir() string {
	const preferred = "logs"
	preferredErr := os.MkdirAll(preferred, 0755)
	if preferredErr == nil {
		return preferred
	}

	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	fallback := filepath.Join(cache, "thanos-mcp", "logs")
	if err := os.MkdirAll(fallback, 0755); err != nil {
		fallback = filepath.Join(os.TempDir(), "thanos-mcp-logs")
		if err := os.MkdirAll(fallback, 0755); err != nil {
			log.Fatalf("thanos-mcp: 无法创建日志目录（首选 ./logs: %v; 回退: %v）", preferredErr, err)
		}
	}
	log.Printf("thanos-mcp: 工作目录无法创建 logs（%v），改用日志目录: %s", preferredErr, fallback)
	return fallback
}

func initLogger() *utils.Logger {
	logDir := os.Getenv("MCP_LOG_DIR")
	if logDir == "" {
		logDir = defaultLogDir()
	}

	levelStr := os.Getenv("MCP_LOG_LEVEL")
	if levelStr == "" {
		levelStr = "info"
	}

	logger, err := utils.NewLogger(logDir, utils.ParseLogLevel(levelStr))
	if err != nil {
		log.Fatalf("初始化日志器失败: %v", err)
	}

	logger.Infof("日志系统初始化完成 | 日志目录: %s | 日志级别: %s", logDir, logger.GetLevel().String())
	return logger
}
