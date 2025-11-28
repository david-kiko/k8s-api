#!/bin/bash

# 测试创建环境接口
ACTION="${1:-create}"
ENV_NAME="${2:-demo}"
NAMESPACE="${3:-}"

echo "使用说明："
echo "  $0 <action> <环境名> [namespace]"
echo "  action选项:"
echo "    create  - 创建环境 (默认)"
echo "    delete  - 删除环境"
echo "    get     - 获取环境信息"
echo "  例如："
echo "    $0 create demo dev-space     # 在dev-space中创建环境"
echo "    $0 delete demo dev-space     # 删除dev-space中的环境"
echo "    $0 get demo dev-space        # 获取dev-space中环境的状态"
echo ""

echo "🚀 测试环境管理接口"
echo "================================"
echo "操作: $ACTION"
echo "环境名: $ENV_NAME"
[ ! -z "$NAMESPACE" ] && echo "Namespace: $NAMESPACE" || echo "Namespace: default (默认)"
echo ""
echo "💡 注意：此接口使用传入kubeconfig中的ServiceAccount身份"
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

case $ACTION in
    create)
        # 构建创建环境的请求体（不包含sa_name，从kubeconfig中提取）
        NAMESPACE_FIELD=""
        if [ ! -z "$NAMESPACE" ]; then
            NAMESPACE_FIELD="\"namespace\": \"$NAMESPACE\","
        fi

        REQUEST_BODY=$(cat << EOF
{
  "kubeconfig": "$KUBECONFIG_CONTENT",
  "name": "$ENV_NAME",
  $NAMESPACE_FIELD
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
EOF
        )

        echo "🔧 创建环境请求体已创建"

        # 调用创建API
        curl -X POST "http://localhost:8080/api/v1/k8s/environments" \
          -H "Content-Type: application/json" \
          -d "$REQUEST_BODY" | jq .
        ;;

    delete)
        # 构建删除环境的请求体
        REQUEST_BODY=$(cat << EOF
{
  "kubeconfig": "$KUBECONFIG_CONTENT",
  "namespace": "$NAMESPACE"
}
EOF
        )

        echo "🗑️ 删除环境请求体已创建"

        # 调用删除API
        curl -X DELETE "http://localhost:8080/api/v1/k8s/environments/$ENV_NAME" \
          -H "Content-Type: application/json" \
          -d "$REQUEST_BODY" | jq .
        ;;

    get)
        # 构建获取环境信息的请求体
        REQUEST_BODY=$(cat << EOF
{
  "kubeconfig": "$KUBECONFIG_CONTENT",
  "namespace": "$NAMESPACE"
}
EOF
        )

        echo "🔍 获取环境信息请求体已创建"

        # 调用获取API
        curl -X GET "http://localhost:8080/api/v1/k8s/environments/$ENV_NAME" \
          -H "Content-Type: application/json" \
          -d "$REQUEST_BODY" | jq .
        ;;

    *)
        echo "❌ 不支持的操作: $ACTION"
        echo "支持的操作: create, delete, get"
        exit 1
        ;;
esac

echo ""
echo "✅ 调用完成！"