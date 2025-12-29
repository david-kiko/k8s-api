# 多阶段构建 Dockerfile
# 第一阶段：构建阶段
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的工具
RUN apk add --no-cache git ca-certificates

# 安装swag工具
RUN go install github.com/swaggo/swag/cmd/swag@latest

# 复制go mod文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY ./code ./code
COPY ./go.mod ./go.mod
COPY ./go.sum ./go.sum

# 生成Swagger文档
RUN swag init -g code/main.go -o docs/swagger

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o k8s-api ./code

# 第二阶段：运行阶段
FROM alpine:latest

# 安装ca证书
RUN apk --no-cache add ca-certificates

# 创建非root用户
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# 设置工作目录
WORKDIR /app

# 从构建阶段复制可执行文件和swagger文档
COPY --from=builder /app/k8s-api .
COPY --from=builder /app/docs/swagger ./docs/swagger

# 创建必要的目录
RUN mkdir -p logs config && \
    chown -R appuser:appgroup /app

# 切换到非root用户
USER appuser

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 设置环境变量
ENV PORT=8080
ENV GIN_MODE=release
# NODE_IP 需要在运行时通过 docker-compose.yml 或 docker run 命令设置

# 启动应用
CMD ["./k8s-api"]