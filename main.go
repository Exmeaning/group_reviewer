package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/driver"

	// 注册进群审核插件
	_ "group_reviewer/plugin"
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	level := os.Getenv("LOG_LEVEL")
	switch strings.ToLower(level) {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}
}

func main() {
	// ── 健康检查模式（供 Docker HEALTHCHECK 调用）──────────────
	// 用法：group_reviewer --health
	// 进程正常 → 退出码 0；否则退出码 1
	if len(os.Args) > 1 && os.Args[1] == "--health" {
		fmt.Println("ok")
		os.Exit(0)
	}

	// ── 从环境变量读取运行时配置（方便 Docker / docker-compose 注入）──
	wsMode := getEnv("WS_MODE", "server") // client (正向) 或 server (反向)
	wsURL := getEnv("WS_URL", "ws://127.0.0.1:6700")
	wsPort := getEnv("WS_PORT", "6700") // 反向 WS 监听端口
	wsHost := getEnv("WS_HOST", "0.0.0.0") // 反向 WS 监听地址
	wsToken := getEnv("WS_TOKEN", "")
	botNick := getEnv("BOT_NICK", "Reviewer")
	cmdPrefix := getEnv("CMD_PREFIX", "/")

	superUsersRaw := getEnv("SUPER_USERS", "")
	var superUsers []int64
	for _, s := range strings.Split(superUsersRaw, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if uid, err := strconv.ParseInt(s, 10, 64); err == nil {
			superUsers = append(superUsers, uid)
		}
	}

	// ── 根据模式选择驱动 ──
	var drivers []zero.Driver
	switch strings.ToLower(wsMode) {
	case "client":
		// 正向 WebSocket 客户端模式：主动连接 OneBot 后端
		log.Infof("[main] 模式: 正向 WS 客户端, 连接端点: %s", wsURL)
		drivers = []zero.Driver{driver.NewWebSocketClient(wsURL, wsToken)}
	default:
		// 反向 WebSocket 服务端模式：等待 OneBot 客户端连接
		listenAddr := wsHost + ":" + wsPort
		log.Infof("[main] 模式: 反向 WS 服务端, 监听地址: %s", listenAddr)
		drivers = []zero.Driver{driver.NewWebSocketServer(1, listenAddr, wsToken)}
	}
	log.Infof("[main] 超级用户: %v", superUsers)

	zero.RunAndBlock(&zero.Config{
		NickName:      []string{botNick},
		CommandPrefix: cmdPrefix,
		SuperUsers:    superUsers,
		Driver:        drivers,
	}, nil)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
