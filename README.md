# K8S Resource API

Kubernetes资源管理API，支持动态创建开发环境和ServiceAccount权限管理。

## 功能特性

- 🚀 基于YAML模板创建完整的K8s开发环境
- 🔐 ServiceAccount权限管理和kubeconfig生成
- 📚 集成Swagger API文档
- 🔄 支持动态传入kubeconfig，无需预设配置
- 🛡️ 完整的错误处理和参数验证

## API接口

### 1. 创建ServiceAccount

**POST** `/api/v1/k8s/service-accounts`

创建ServiceAccount并返回对应的kubeconfig。如果SA不存在则自动创建并分配权限。

#### 请求参数

```json
{
  "kubeconfig": "apiVersion: v1\nkind: Config\n...",
  "sa_name": "dev-user",
  "namespace": "default",
  "create_if_not_exists": true,
  "resource_limits": {
    "cpu": "8",
    "memory": "16Gi",
    "storage": "20Gi",
    "pod_count": "2"
  }
}
```

**参数说明：**

- `kubeconfig`: **必需** - 用于操作的kubeconfig内容
- `sa_name`: **必需** - ServiceAccount名称
- `namespace`: **可选** - SA所在的namespace，默认为"default"
- `create_if_not_exists`: **可选** - 是否自动创建不存在的namespace和SA，默认为true
- `resource_limits`: **可选** - 资源限制配置，字段不填时使用默认值

  **resource_limits字段说明：**
  - `cpu`: CPU限制，**默认：8**，例如：`"8"` (8个CPU)、`"4000m"` (4个CPU，1000m=1CPU)
  - `memory`: 内存限制，**默认：16Gi**，例如：`"16Gi"` (16Gib)、`"16000Mi"` (16Gib，1024Mi=1Gi)
  - `storage`: 存储限制，**默认：20Gi**，例如：`"20Gi"` (20Gib)、`"20480Mi"` (20Gib)
  - `pod_count`: Pod数量限制，**默认：2**，例如：`"2"` (最多2个Pod)

#### 响应示例

```json
{
  "success": true,
  "message": "kubeconfig生成成功",
  "data": "apiVersion: v1\nkind: Config\n..."
}
```

### 2. 删除ServiceAccount

**DELETE** `/api/v1/k8s/service-accounts/{name}`

删除指定的ServiceAccount及其相关权限配置（Role、RoleBinding、ResourceQuota）。

#### 请求参数

```json
{
  "kubeconfig": "apiVersion: v1\nkind: Config\n...",
  "namespace": "dev-space"
}
```

#### 响应示例

```json
{
  "success": true,
  "message": "ServiceAccount删除成功",
  "data": {
    "service_account": "dev-user",
    "namespace": "dev-space"
  }
}
```

### 4. 创建环境

**POST** `/api/v1/k8s/environments`

根据传入的参数创建完整的K8s环境，包括：
- PersistentVolumeClaim (workspace + vscode存储)
- Deployment (开发环境容器)
- Service (NodePort暴露服务)

#### 请求参数

```json
{
  "kubeconfig": "apiVersion: v1\nkind: Config\n...",
  "name": "demo",
  "namespace": "dev-space",
  "resources": {
    "cpu": "1000m",
    "cpu_limit": "2000m",
    "memory": "2Gi",
    "memory_limit": "4Gi"
  },
  "storage": {
    "workspace": "10Gi",
    "vscode": "5Gi"
  },
  "nodeports": {
    "vscode": 30582,
    "ssh": 31496,
    "terminal": 32259
  }
}
```

**参数说明：**

- `kubeconfig`: **必需** - ServiceAccount的kubeconfig，用于身份验证和操作权限
- `name`: **必需** - 环境名称
- `namespace`: **可选** - 环境所在的namespace，默认使用kubeconfig中指定的namespace
- `resources`: **必需** - CPU和内存资源配置
  - `cpu`: CPU请求量，例如：`"1000m"` (1个CPU)
  - `cpu_limit`: CPU限制量，例如：`"2000m"` (2个CPU)
  - `memory`: 内存请求量，例如：`"2Gi"` (2Gib)
  - `memory_limit`: 内存限制量，例如：`"4Gi"` (4Gib)
- `storage`: **必需** - 存储配置
  - `workspace`: 工作空间存储大小，例如：`"10Gi"`
  - `vscode`: VS Code扩展存储大小，例如：`"5Gi"`
- `nodeports`: **必需** - NodePort端口配置
  - `vscode`: VS Code Web界面端口
  - `ssh`: SSH服务端口
  - `terminal`: Web终端端口

#### 响应示例

```json
{
  "success": true,
  "message": "环境创建成功",
  "data": {
    "namespace": "dev-space",
    "service_name": "demo-service",
    "sa_name": "dev-user"
  }
}
```

### 5. 删除环境

**DELETE** `/api/v1/k8s/environments/{name}`

删除指定名称的环境，包括Deployment、Service和PVC。

#### 请求参数

```json
{
  "kubeconfig": "apiVersion: v1\nkind: Config\n...",
  "namespace": "dev-space"
}
```

#### 响应示例

```json
{
  "success": true,
  "message": "环境删除成功"
}
```

### 6. 获取环境信息

**GET** `/api/v1/k8s/environments/{name}`

获取指定名称的环境的状态信息。

#### 请求参数

```json
{
  "kubeconfig": "apiVersion: v1\nkind: Config\n...",
  "namespace": "dev-space"
}
```

#### 响应示例

```json
{
  "success": true,
  "message": "环境信息获取成功",
  "data": {
    "deployment": {
      "exists": true,
      "replicas": 1,
      "ready": true
    },
    "service": {
      "exists": true,
      "type": "NodePort",
      "clusterIP": "10.96.0.123"
    },
    "pvcs": {
      "demo-workspace": {
        "exists": true,
        "status": "Bound",
        "storage": "10Gi"
      },
      "demo-vscode": {
        "exists": true,
        "status": "Bound",
        "storage": "5Gi"
      }
    }
  }
}
```

## 快速开始

### 方式一：使用Docker Compose（推荐）

```bash
# 启动服务
cd docker && docker-compose up -d --build

# 查看日志
cd docker && docker-compose logs -f k8s-api

# 停止服务
cd docker && docker-compose down
```

或者使用Makefile：

```bash
make docker-up  # 启动服务
make docker-down  # 停止服务
make docker-logs  # 查看日志
```

### 方式二：本地开发

```bash
# 1. 安装依赖
go mod tidy

# 2. 生成Swagger文档
swag init -g code/main.go

# 3. 运行服务
go run code/main.go code/handlers.go code/utils.go
```

或者使用Makefile：

```bash
make build    # 构建应用
make run      # 运行应用
make swag     # 生成文档
```

### 3. 查看API文档

访问 `http://localhost:8080/swagger/index.html` 查看交互式API文档。

## 示例用法

### 1. 创建ServiceAccount示例

```bash
# 使用默认资源限制（8CPU/16Gi内存/20Gi存储/2个Pod）
curl -X POST "http://localhost:8080/api/v1/k8s/service-accounts" \
  -H "Content-Type: application/json" \
  -d '{
    "kubeconfig": "...",
    "sa_name": "dev-user",
    "namespace": "my-space",
    "create_if_not_exists": true,
    "resource_limits": {
      "cpu": "8",
      "memory": "16Gi",
      "storage": "20Gi",
      "pod_count": "2"
    }
  }'

# 部分使用默认值（只修改CPU和内存，其他使用默认值）
curl -X POST "http://localhost:8080/api/v1/k8s/service-accounts" \
  -H "Content-Type: application/json" \
  -d '{
    "kubeconfig": "...",
    "sa_name": "dev-user",
    "namespace": "my-space",
    "create_if_not_exists": true,
    "resource_limits": {
      "cpu": "2000m",
      "memory": "4Gi"
    }
  }'
# 结果：CPU:2000m, 内存:4Gi, 存储:20Gi(默认), Pod数量:2(默认)

# 或使用提供的测试脚本
chmod +x scripts/test-service-account.sh
cd scripts && ./test-service-account.sh dev-user my-space 2000m 4Gi
```

### 2. 删除ServiceAccount示例

```bash
# 删除ServiceAccount及其权限配置
curl -X DELETE "http://localhost:8080/api/v1/k8s/service-accounts/dev-user" \
  -H "Content-Type: application/json" \
  -d '{
    "kubeconfig": "...",
    "namespace": "my-space"
  }'

# 或使用测试脚本
cd scripts && ./test-service-account.sh delete dev-user my-space
```

### 4. 创建环境示例

```bash
# 创建环境（使用传入的kubeconfig中的ServiceAccount身份）
curl -X POST http://localhost:8080/api/v1/k8s/environments \
  -H "Content-Type: application/json" \
  -d '{
    "kubeconfig": "...",
    "name": "my-demo",
    "namespace": "my-space",
    "resources": {
      "cpu": "1000m",
      "cpu_limit": "2000m",
      "memory": "2Gi",
      "memory_limit": "4Gi"
    },
    "storage": {
      "workspace": "20Gi",
      "vscode": "10Gi"
    },
    "nodeports": {
      "vscode": 30888,
      "ssh": 32222,
      "terminal": 33333
    }
  }'

# 或使用提供的测试脚本
chmod +x scripts/test-environments.sh
cd scripts && ./test-environments.sh create demo dev-space
```

**使用流程：**
```bash
# 1. 创建ServiceAccount并获取kubeconfig
./scripts/test-service-account.sh create dev-user dev-space

# 2. 使用获取的kubeconfig创建环境（环境中的Pod会以dev-user身份运行）
./scripts/test-environments.sh create demo dev-space
```

### 5. 删除环境示例

```bash
# 删除环境
curl -X DELETE "http://localhost:8080/api/v1/k8s/environments/demo" \
  -H "Content-Type: application/json" \
  -d '{
    "kubeconfig": "...",
    "namespace": "dev-space"
  }'

# 或使用测试脚本
cd scripts && ./test-environments.sh delete demo dev-space
```

### 6. 获取环境信息示例

```bash
# 获取环境状态
curl -X GET "http://localhost:8080/api/v1/k8s/environments/demo" \
  -H "Content-Type: application/json" \
  -d '{
    "kubeconfig": "...",
    "namespace": "dev-space"
  }'

# 或使用测试脚本
cd scripts && ./test-environments.sh get demo dev-space
```

## 项目结构

```
.
├── code/                    # 源代码目录
│   ├── main.go             # 主程序入口和路由配置
│   ├── handlers.go         # HTTP请求处理函数
│   └── utils.go            # 工具函数（YAML生成、kubeconfig生成等）
├── docker/                 # Docker相关文件
│   ├── Dockerfile          # Docker镜像构建文件
│   ├── docker-compose.yml  # Docker Compose配置文件
│   └── demo-service.yaml   # K8s服务模板
├── scripts/                # 脚本文件
│   ├── test-service-account.sh  # ServiceAccount测试脚本
│   ├── test-environments.sh      # 环境管理测试脚本
│   └── test-kubeconfig-post.sh   # 旧版kubeconfig测试脚本（兼容）
├── docs/                   # 文档目录
│   └── swagger/            # Swagger生成的API文档
│       ├── docs.go
│       ├── swagger.json
│       └── swagger.yaml
├── bin/                    # 编译输出目录
├── logs/                   # 日志目录
├── .claude/               # Claude配置
├── go.mod                 # Go模块依赖
├── go.sum                 # Go依赖锁定文件
├── Makefile               # 构建和部署脚本
├── docker-compose.md      # Docker Compose部署文档
└── README.md              # 项目说明文档
```

## 依赖说明

- `gin-gonic/gin`: HTTP Web框架
- `swaggo/gin-swagger`: Swagger文档集成
- `k8s.io/client-go`: Kubernetes官方客户端
- `sigs.k8s.io/yaml`: YAML处理库

## 注意事项

1. **kubeconfig安全性**: 请确保传入的kubeconfig具有足够的权限来创建资源
2. **端口冲突**: 使用NodePort时请确保端口未被占用
3. **存储类**: 默认使用`local`存储类，请确保集群支持
4. **权限管理**: 生成的SA将获得指定namespace的完整权限

## 许可证

Apache License 2.0