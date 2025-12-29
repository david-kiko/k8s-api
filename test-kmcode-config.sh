#!/bin/bash

# 测试创建环境时传入 KMCODE_CONFIG

# 设置变量
API_URL="http://localhost:8080"
ENV_NAME="test-kmcode-$(date +%s)"
NAMESPACE="docker-test-ns"

# 读取测试用的小 kubeconfig
KUBECONFIG_FILE="./test-sa-with-valid-token.yaml"
if [ ! -f "$KUBECONFIG_FILE" ]; then
    echo "错误: 找不到测试用的 kubeconfig 文件: $KUBECONFIG_FILE"
    echo "请先运行之前的测试脚本生成该文件"
    exit 1
fi

# 读取 kubeconfig 内容
KUBECONFIG_CONTENT=$(cat "$KUBECONFIG_FILE")

# 构建请求数据
REQUEST_DATA=$(cat <<EOF
{
    "kubeconfig": $(echo "$KUBECONFIG_CONTENT" | jq -Rs .),
    "name": "$ENV_NAME",
    "namespace": "$NAMESPACE",
    "image": "registry.opsman.top/kmai/ubuntu:22.04-ide",
    "resources": {
        "cpu": "500m",
        "cpu_limit": "1000m",
        "memory": "1Gi",
        "memory_limit": "2Gi"
    },
    "storage": {
        "workspace": "5Gi",
        "vscode": "2Gi"
    },
    "nodeports": {
        "vscode": 0,
        "ssh": 0,
        "terminal": 0
    },
    "kmcode_config": {
        "api_url": "https://api.kmcode.com",
        "debug": true,
        "timeout": 30,
        "features": ["feature1", "feature2"],
        "database": {
            "host": "db.kmcode.com",
            "port": 5432
        }
    }
}
EOF
)

echo "创建环境测试: $ENV_NAME"
echo "请求内容包含 KMCODE_CONFIG 配置..."

# 发送请求
echo "发送请求到: $API_URL/environments"
RESPONSE=$(curl -s -X POST "$API_URL/environments" \
    -H "Content-Type: application/json" \
    -d "$REQUEST_DATA")

# 检查响应
echo -e "\n响应:"
echo "$RESPONSE" | jq '.'

# 检查是否成功
SUCCESS=$(echo "$RESPONSE" | jq -r '.success')
if [ "$SUCCESS" = "true" ]; then
    echo -e "\n✅ 环境创建成功!"

    # 获取环境信息
    sleep 5  # 等待环境创建

    GET_REQUEST_DATA=$(cat <<EOF
{
    "kubeconfig": $(echo "$KUBECONFIG_CONTENT" | jq -Rs .),
    "namespace": "$NAMESPACE"
}
EOF
)

    echo -e "\n获取环境信息..."
    ENV_RESPONSE=$(curl -s -X POST "$API_URL/environments/$ENV_NAME" \
        -H "Content-Type: application/json" \
        -d "$GET_REQUEST_DATA")

    echo -e "\n环境信息:"
    echo "$ENV_RESPONSE" | jq '.'

    # 清理：删除测试环境
    echo -e "\n清理测试环境..."
    DELETE_REQUEST_DATA=$(cat <<EOF
{
    "kubeconfig": $(echo "$KUBECONFIG_CONTENT" | jq -Rs .),
    "namespace": "$NAMESPACE"
}
EOF
)

    DELETE_RESPONSE=$(curl -s -X DELETE "$API_URL/environments/$ENV_NAME" \
        -H "Content-Type: application/json" \
        -d "$DELETE_REQUEST_DATA")

    echo "删除响应:"
    echo "$DELETE_RESPONSE" | jq '.'

else
    echo -e "\n❌ 环境创建失败!"
    exit 1
fi