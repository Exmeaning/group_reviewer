# ╔══════════════════════════════════════════════════════════════╗
# ║          group_reviewer — ZeroBot 进群审核插件               ║
# ║  多阶段构建：builder(Go 1.24) → 最终镜像(distroless/static)  ║
# ╚══════════════════════════════════════════════════════════════╝

# ──────────────────────────────────────────────
# Stage 1: 构建阶段
# ──────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

# 安装 git（go mod download 可能需要）和 ca-certificates（HTTPS 拉包）
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# ── 优先复制依赖描述文件，充分利用 Docker 层缓存 ──
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# ── 复制全部源码 ──
COPY . .

# ── 静态编译，裁掉所有 CGO 和调试符号 ──
RUN CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /out/group_reviewer \
      .

# ──────────────────────────────────────────────
# Stage 2: 最终运行镜像（最小化攻击面）
# ──────────────────────────────────────────────
FROM gcr.io/distroless/static:nonroot

# 从 builder 阶段复制必要文件
COPY --from=builder /out/group_reviewer  /app/group_reviewer
COPY --from=builder /usr/share/zoneinfo  /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

WORKDIR /app

# ──────────────────────────────────────────────
# 运行时环境变量（均可通过 -e 或 docker-compose 覆盖）
# ──────────────────────────────────────────────

# OneBot 正向 WebSocket 地址（NapCat / go-cqhttp / Lagrange 等）
ENV WS_URL="ws://onebot:6700"

# OneBot access_token（若未设置则留空）
ENV WS_TOKEN=""

# Bot 昵称（用于 @机器人 触发）
ENV BOT_NICK="Reviewer"

# 命令前缀（默认 /）
ENV CMD_PREFIX="/"

# 超级用户 QQ 号，逗号分隔，例如 "123456,789012"
ENV SUPER_USERS=""

# 时区
ENV TZ="Asia/Shanghai"

# 日志级别：debug / info / warn / error
ENV LOG_LEVEL="info"

# ──────────────────────────────────────────────
# 健康检查：每 30s 确认进程存活
# ──────────────────────────────────────────────
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/app/group_reviewer", "--health"] || exit 1

ENTRYPOINT ["/app/group_reviewer"]
