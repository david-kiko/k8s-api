#!/bin/bash

# 测试ServiceAccount接口
ACTION="${1:-create}"
SERVICE_ACCOUNT_NAME="${2:-test-user}"
NAMESPACE="${3:-default}"
CPU="${4:-}"
MEMORY="${5:-}"
STORAGE="${6:-}"
POD_COUNT="${7:-}"

echo "使用说明："
echo "  $0 <action> <服务账户> <namespace> <CPU> <内存> <存储> <Pod数量>"
echo "  action选项:"
echo "    create  - 创建ServiceAccount (默认)"
echo "    delete  - 删除ServiceAccount"
echo "  例如："
echo "    $0 create test-user david 2000m 4Gi     # 创建SA并设置资源限制"
echo "    $0 delete test-user david               # 删除SA"
echo ""

echo "🚀 测试ServiceAccount接口"
echo "================================"
echo "操作: $ACTION"
echo "ServiceAccount: $SERVICE_ACCOUNT_NAME"
echo "Namespace: $NAMESPACE"

if [ "$ACTION" = "create" ]; then
# 显示资源限制配置信息
echo ""
echo "📊 资源限制配置:"
[ ! -z "$CPU" ] && echo "  - CPU: $CPU" || echo "  - CPU: 8 (默认)"
[ ! -z "$MEMORY" ] && echo "  - 内存: $MEMORY" || echo "  - 内存: 16Gi (默认)"
[ ! -z "$STORAGE" ] && echo "  - 存储: $STORAGE" || echo "  - 存储: 20Gi (默认)"
[ ! -z "$POD_COUNT" ] && echo "  - Pod数量: $POD_COUNT" || echo "  - Pod数量: 2 (默认)"
echo ""

# 读取kubeconfig文件内容（如果存在）
if [ -f "./config.yaml" ]; then
    echo "📁 从config.yaml文件读取kubeconfig"
    # 读取文件并转义为JSON字符串格式
    KUBECONFIG_CONTENT=$(cat "./config.yaml" | sed ':a;N;$!ba;s/\n/\\n/g' | sed 's/"/\\"/g')
else
    echo "❌ 未找到config.yaml文件"
    exit 1
fi

# 构建JSON请求体
REQUEST_BODY=$(cat << EOF
{
  "kubeconfig": "$KUBECONFIG_CONTENT",
  "sa_name": "$SERVICE_ACCOUNT_NAME",
  "namespace": "$NAMESPACE",
  "create_if_not_exists": true
EOF
)

# 如果有资源限制参数，添加到请求体中
if [ ! -z "$CPU" ] || [ ! -z "$MEMORY" ] || [ ! -z "$STORAGE" ] || [ ! -z "$POD_COUNT" ]; then
    REQUEST_BODY="$REQUEST_BODY,"
    REQUEST_BODY="$REQUEST_BODY  \"resource_limits\": {"

    # 动态添加资源限制字段
    FIRST=true
    if [ ! -z "$CPU" ]; then
        if [ "$FIRST" = false ]; then REQUEST_BODY="$REQUEST_BODY,"; fi
        REQUEST_BODY="$REQUEST_BODY    \"cpu\": \"$CPU\""
        FIRST=false
    fi

    if [ ! -z "$MEMORY" ]; then
        if [ "$FIRST" = false ]; then REQUEST_BODY="$REQUEST_BODY,"; fi
        REQUEST_BODY="$REQUEST_BODY    \"memory\": \"$MEMORY\""
        FIRST=false
    fi

    if [ ! -z "$STORAGE" ]; then
        if [ "$FIRST" = false ]; then REQUEST_BODY="$REQUEST_BODY,"; fi
        REQUEST_BODY="$REQUEST_BODY    \"storage\": \"$STORAGE\""
        FIRST=false
    fi

    if [ ! -z "$POD_COUNT" ]; then
        if [ "$FIRST" = false ]; then REQUEST_BODY="$REQUEST_BODY,"; fi
        REQUEST_BODY="$REQUEST_BODY    \"pod_count\": \"$POD_COUNT\""
        FIRST=false
    fi

    REQUEST_BODY="$REQUEST_BODY"
    REQUEST_BODY="$REQUEST_BODY  }"
fi

REQUEST_BODY="$REQUEST_BODY}"
echo "$REQUEST_BODY" > request.json

echo "🔧 创建ServiceAccount请求体已创建"

# 调用API
curl -X POST "http://localhost:8080/api/v1/k8s/service-accounts" \
  -H "Content-Type: application/json" \
  -d @request.json | jq .

elif [ "$ACTION" = "delete" ]; then
echo ""

# 读取kubeconfig文件内容（如果存在）
if [ -f "./config.yaml" ]; then
    echo "📁 从config.yaml文件读取kubeconfig"
    # 读取文件并转义为JSON字符串格式
    KUBECONFIG_CONTENT=$(cat "./config.yaml" | sed ':a;N;$!ba;s/\n/\\n/g' | sed 's/"/\\"/g')
else
    echo "❌ 未找到config.yaml文件"
    exit 1
fi

# 构建删除请求体
REQUEST_BODY=$(cat << EOF
{
  "kubeconfig": "$KUBECONFIG_CONTENT",
  "namespace": "$NAMESPACE"
}
EOF
)

echo "$REQUEST_BODY" > request.json

echo "🗑️ 删除ServiceAccount请求体已创建"

# 调用删除API
curl -X DELETE "http://localhost:8080/api/v1/k8s/service-accounts/$SERVICE_ACCOUNT_NAME" \
  -H "Content-Type: application/json" \
  -d @request.json | jq .

else
echo ""
echo "❌ 不支持的操作: $ACTION"
echo "支持的操作: create, delete"
exit 1
fi

echo ""
echo "✅ 调用完成！"

# 清理临时文件
rm -f request.json