#!/bin/bash

# 使用POST请求体方式调用kubeconfig接口
SERVICE_ACCOUNT_NAME="${1:-test-user}"
NAMESPACE="${2:-default}"
CPU="${3:-}"
MEMORY="${4:-}"
STORAGE="${5:-}"
POD_COUNT="${6:-}"

echo "使用说明："
echo "  $0 <服务账户> <namespace> <CPU> <内存> <存储> <Pod数量>"
echo "  例如："
echo "    $0 test-user david                 # 使用默认值(8CPU/16Gi/20Gi/2Pod)"
echo "    $0 test-user david 2000m 4Gi       # 只设置CPU和内存，其他使用默认值"
echo "    $0 test-user david 2 4Gi 10Gi 10   # 设置所有资源"
echo ""

echo "🚀 调用kubeconfig接口（POST方式）"
echo "================================"
echo "ServiceAccount: $SERVICE_ACCOUNT_NAME"
echo "Namespace: $NAMESPACE"

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
  "namespace": "$NAMESPACE",
  "create_if_not_exists": true
EOF
)

# 如果有资源限制参数，添加到请求体中
if [ ! -z "$CPU" ] || [ ! -z "$MEMORY" ] || [ ! -z "$STORAGE" ] || [ ! -z "$POD_COUNT" ]; then
    REQUEST_BODY="$REQUEST_BODY,"
    REQUEST_BODY="$REQUEST_BODY"
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

echo "🔧 请求体已创建"
echo ""

# 调用API
curl -X POST "http://localhost:8080/api/v1/k8s/kubeconfig/${SERVICE_ACCOUNT_NAME}" \
  -H "Content-Type: application/json" \
  -d @request.json | jq .

echo ""
echo "✅ 调用完成！"

# 清理临时文件
rm -f request.json