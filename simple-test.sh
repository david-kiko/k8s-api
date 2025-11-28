#!/bin/bash

echo "🚀 直接使用config文件测试API"
echo "==========================="

# 使用Python来正确处理多行字符串和JSON转义
python3 << 'EOF'
import json
import requests
import subprocess

# 读取kubeconfig文件
with open('D:/work/k8s/api/config', 'r') as f:
    kubeconfig_content = f.read()

# 构造SA创建请求
sa_request = {
    "kubeconfig": kubeconfig_content,
    "sa_name": "docker-test-sa",
    "namespace": "docker-test-ns",
    "create_if_not_exists": True,
    "token_expiration_hours": 2,
    "resource_limits": {
        "cpu": "2",
        "memory": "4Gi",
        "storage": "10Gi",
        "pod_count": "3"
    }
}

print("📨 发送CreateServiceAccount请求...")
print(f"请求大小: {len(json.dumps(sa_request))} 字节")

try:
    response = requests.post(
        "http://localhost:8080/api/v1/k8s/service-accounts",
        headers={"Content-Type": "application/json"},
        json=sa_request,
        timeout=30
    )

    print(f"📬 响应状态码: {response.status_code}")
    print(f"📬 响应内容: {response.text}")

    if response.status_code == 200:
        result = response.json()
        if result.get('success') and result.get('data'):
            sa_kubeconfig = result['data']
            print(f"✅ 成功获取SA kubeconfig，长度: {len(sa_kubeconfig)}")

            # 构造环境创建请求
            env_request = {
                "kubeconfig": sa_kubeconfig,
                "name": "docker-test-env",
                "namespace": "docker-test-ns",
                "resources": {
                    "cpu": "1000m",
                    "cpu_limit": "2000m",
                    "memory": "2Gi",
                    "memory_limit": "4Gi"
                },
                "storage": {
                    "workspace": "8Gi",
                    "vscode": "3Gi"
                },
                "nodeports": {
                    "vscode": 30888,
                    "ssh": 32222,
                    "terminal": 32766
                }
            }

            print("\n📨 发送CreateEnvironment请求...")

            env_response = requests.post(
                "http://localhost:8080/api/v1/k8s/environments",
                headers={"Content-Type": "application/json"},
                json=env_request,
                timeout=30
            )

            print(f"📬 环境创建响应状态码: {env_response.status_code}")
            print(f"📬 环境创建响应内容: {env_response.text}")

            if env_response.status_code == 200:
                print("✅ 完整流程测试成功！")
            else:
                print("❌ 环境创建失败")
        else:
            print("❌ SA创建失败，无法获取kubeconfig")
    else:
        print("❌ SA创建请求失败")

except requests.exceptions.RequestException as e:
    print(f"❌ 请求异常: {e}")
except Exception as e:
    print(f"❌ 其他错误: {e}")

EOF