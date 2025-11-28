#!/bin/bash

# K8S Resource API 测试脚本

BASE_URL="http://localhost:8080"
KUBECONFIG_CONTENT=$(cat ../.kube/config 2>/dev/null || cat ~/.kube/config)

echo "🚀 测试 K8S Resource API"
echo "========================="

# 1. 健康检查
echo "1. 健康检查..."
curl -s "$BASE_URL/health" | jq .

echo ""
echo "2. 创建开发环境..."
CREATE_RESPONSE=$(curl -s -X POST "$BASE_URL/api/v1/k8s/dev-env" \
  -H "Content-Type: application/json" \
  -d "{
    \"kubeconfig\": $(echo "$KUBECONFIG_CONTENT" | jq -Rsa .),
    \"sa_name\": \"test-user\",
    \"name\": \"test-demo\",
    \"namespace\": \"test-space\",
    \"resources\": {
      \"cpu\": \"500m\",
      \"cpu_limit\": \"1000m\",
      \"memory\": \"1Gi\",
      \"memory_limit\": \"2Gi\"
    },
    \"storage\": {
      \"workspace\": \"5Gi\",
      \"vscode\": \"2Gi\"
    },
    \"nodeports\": {
      \"vscode\": 30888,
      \"ssh\": 32222,
      \"terminal\": 33333
    }
  }")

echo "$CREATE_RESPONSE" | jq .

echo ""
echo "3. 获取SA的kubeconfig..."
curl -s "$BASE_URL/api/v1/k8s/kubeconfig/test-user?kubeconfig=$(echo "$KUBECONFIG_CONTENT" | jq -sRr @uri)&namespace=test-space" | jq .

echo ""
echo "✅ 测试完成！"
echo "📚 API文档: $BASE_URL/swagger/index.html"