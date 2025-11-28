.PHONY: help build run test clean docker-build docker-up docker-down docker-logs swag

# 默认目标
help:
	@echo "K8S Resource API - 可用命令:"
	@echo ""
	@echo "本地开发:"
	@echo "  build      - 构建Go应用"
	@echo "  run        - 运行Go应用"
	@echo "  test       - 运行测试"
	@echo "  swag       - 生成Swagger文档"
	@echo ""
	@echo "Docker操作:"
	@echo "  docker-build - 构建Docker镜像"
	@echo "  docker-up    - 启动Docker Compose服务"
	@echo "  docker-down  - 停止Docker Compose服务"
	@echo "  docker-logs  - 查看Docker日志"
	@echo "  clean        - 清理临时文件"
	@echo ""
	@echo "示例:"
	@echo "  make docker-up    # 启动服务"
	@echo "  make docker-logs  # 查看日志"

# 本地开发
build:
	go mod tidy
	go build -o k8s-api ./code

run: build
	./k8s-api

swag:
	swag init -g code/main.go

test:
	@echo "运行测试..."

clean:
	rm -f k8s-api
	rm -rf logs/

# Docker操作
docker-build:
	cd docker && docker-compose build

docker-up:
	cd docker && docker-compose up -d --build
	@echo "✅ 服务已启动"
	@echo "📚 API文档: http://localhost:8080/swagger/index.html"

docker-down:
	cd docker && docker-compose down

docker-logs:
	cd docker && docker-compose logs -f k8s-api

# 快速命令
start: docker-up
stop: docker-down
logs: docker-logs