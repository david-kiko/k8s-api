#!/bin/bash

# 读取kubeconfig文件内容并正确转义
KUBECONFIG_FILE="./config.yaml"
SERVICE_ACCOUNT_NAME="test-user"
NAMESPACE="default"

echo "🚀 调用kubeconfig接口"
echo "===================="

# 读取文件并转义
KUBECONFIG_CONTENT=$(cat "$KUBECONFIG_FILE" | jq -Rsa .)

# 调用API
curl -X GET "http://localhost:8080/api/v1/k8s/kubeconfig/${SERVICE_ACCOUNT_NAME}" \
  -H "Content-Type: application/json" \
  -G \
  --data-urlencode "kubeconfig=${KUBECONFIG_CONTENT}" \
  --data-urlencode "namespace=${NAMESPACE}" \
  --data-urlencode "create_if_not_exists=true" | jq .

echo ""
echo "✅ 调用完成！"