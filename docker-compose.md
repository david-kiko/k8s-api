# Docker Compose 部署指南

## 快速启动

### 1. 构建并启动服务

```bash
# 构建镜像并启动容器
docker-compose up --build

# 后台运行
docker-compose up -d --build
```

### 2. 访问服务

- **API服务**: http://localhost:8080
- **API文档**: http://localhost:8080/swagger/index.html
- **健康检查**: http://localhost:8080/health

## 常用命令

```bash
# 启动服务
docker-compose up -d

# 停止服务
docker-compose down

# 查看日志
docker-compose logs -f k8s-api

# 重启服务
docker-compose restart

# 查看服务状态
docker-compose ps
```

## 配置说明

### 端口配置
默认端口为 `8080`，如需修改：

```yaml
ports:
  - "你的端口:8080"
```

### 日志配置
日志会输出到容器的标准输出，如需持久化日志：

```yaml
volumes:
  - ./logs:/app/logs
```

### 环境变量
支持的环境变量：

- `PORT`: 服务端口（默认: 8080）
- `GIN_MODE`: 运行模式（默认: release）

## 生产环境部署

### 1. 使用自定义配置文件

```bash
docker-compose -f docker-compose.yml up -d
```

### 2. 资源限制

可以在 `docker-compose.yml` 中添加资源限制：

```yaml
services:
  k8s-api:
    deploy:
      resources:
        limits:
          cpus: '1'
          memory: 512M
        reservations:
          cpus: '0.5'
          memory: 256M
```

### 3. 日志管理

配置日志轮转：

```yaml
services:
  k8s-api:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

## 故障排除

### 1. 服务无法启动

```bash
# 检查容器状态
docker-compose ps

# 查看详细日志
docker-compose logs k8s-api
```

### 2. 端口占用

修改 `docker-compose.yml` 中的端口映射：

```yaml
ports:
  - "8081:8080"  # 使用8081端口
```

### 3. 健康检查失败

检查应用是否正常启动：

```bash
curl http://localhost:8080/health
```

## 更新部署

```bash
# 拉取最新代码
git pull

# 重新构建并启动
docker-compose up -d --build
```