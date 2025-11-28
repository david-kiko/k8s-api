# Docker 部署说明

## 挂载管理员 kubeconfig

为了支持 ServiceAccount 管理接口使用管理员权限，可以挂载管理员 kubeconfig 文件：

### 1. 使用 docker-compose

修改 `docker-compose.yml` 中的挂载路径：

```yaml
volumes:
  - ../logs:/app/logs
  # 替换为实际的管理员kubeconfig文件路径
  - /home/user/.kube/config:/app/config/admin-kubeconfig:ro
```

### 2. 使用 docker run

```bash
docker run -d \
  -p 8080:8080 \
  -e NODE_IP=192.168.1.100 \
  -v /home/user/.kube/config:/app/config/admin-kubeconfig:ro \
  --name k8s-api \
  k8s-resource-api:1.0
```

### 3. kubeconfig 文件要求

- 文件必须挂载到 `/app/config/admin-kubeconfig`
- 文件需要包含集群管理权限
- 建议使用只读挂载（`:ro`）

## API 使用说明

### ServiceAccount 接口（可以使用管理员 kubeconfig）

```bash
# 不传入 kubeconfig，使用挂载的管理员 kubeconfig
curl -X POST "http://localhost:8080/api/v1/k8s/service-accounts/dev-user" \
  -H "Content-Type: application/json" \
  -d '{
    "namespace": "dev-space",
    "create_if_not_exists": true,
    "token_expiration_hours": 24
  }'

# 传入自己的 kubeconfig，覆盖管理员 kubeconfig
curl -X POST "http://localhost:8080/api/v1/k8s/service-accounts/dev-user" \
  -H "Content-Type: application/json" \
  -d '{
    "kubeconfig": "用户自己的kubeconfig内容",
    "namespace": "dev-space"
  }'
```

### 环境接口（必须传入用户 kubeconfig）

```bash
# 必须传入 kubeconfig
curl -X POST "http://localhost:8080/api/v1/k8s/environments/demo" \
  -H "Content-Type: application/json" \
  -d '{
    "kubeconfig": "用户的serviceaccount kubeconfig",
    "namespace": "dev-space"
  }'
```

## 权限说明

- **ServiceAccount 管理**：可以使用管理员 kubeconfig 或用户传入的 kubeconfig
- **环境管理**：必须使用用户传入的 kubeconfig（即创建的 ServiceAccount kubeconfig）

这样可以确保用户只能管理自己的环境，而管理员可以管理 ServiceAccount。