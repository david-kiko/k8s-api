package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// generateDeploymentYAML 生成Deployment的YAML
func generateDeploymentYAML(req CreateEnvironmentRequest) string {
	// 动态生成环境变量列表
	envVarsYAML := ""
	if len(req.Env) > 0 {
		envVarsYAML = "        env:\n"
		for _, env := range req.Env {
			envVarsYAML += fmt.Sprintf("        - name: %s\n          value: \"%s\"\n", env.Name, env.Value)
		}
	}

	// 动态生成端口列表
	portsYAML := ""
	for _, port := range req.Ports {
		portsYAML += fmt.Sprintf("        - containerPort: %d  # %s\n", port.ContainerPort, port.Name)
	}

	// 动态生成存储卷列表
	volumeMountsYAML := ""
	volumesYAML := ""
	for _, storage := range req.Storage {
		volumeMountsYAML += fmt.Sprintf("        - name: %s\n          mountPath: %s\n", storage.Name+"-storage", storage.Path)
		volumesYAML += fmt.Sprintf("      - name: %s\n        persistentVolumeClaim:\n          claimName: %s-%s\n", storage.Name+"-storage", req.Name, storage.Name)
	}

	// 生成健康检查探针
	probeYAML := ""
	if req.HealthCheck != nil && req.HealthCheck.Enabled {
		// 设置默认值
		initialDelay := req.HealthCheck.InitialDelay
		periodSeconds := req.HealthCheck.PeriodSeconds
		if periodSeconds == 0 {
			periodSeconds = 10
		}
		failureThreshold := req.HealthCheck.FailureThreshold
		if failureThreshold == 0 {
			failureThreshold = 3
		}

		// 获取目标端口号
		targetPort := ""
		for _, port := range req.Ports {
			if port.Name == req.HealthCheck.PortName {
				targetPort = fmt.Sprintf("%d", port.ContainerPort)
				break
			}
		}
		if targetPort == "" {
			targetPort = req.HealthCheck.PortName // 可能是端口号
		}

		probeYAML = fmt.Sprintf(`        livenessProbe:
          httpGet:
            path: %s
            port: %s
          initialDelaySeconds: %d
          periodSeconds: %d
          failureThreshold: %d
        readinessProbe:
          httpGet:
            path: %s
            port: %s
          initialDelaySeconds: %d
          periodSeconds: %d
          failureThreshold: %d
`,
			req.HealthCheck.Path, targetPort, initialDelay, periodSeconds, failureThreshold,
			req.HealthCheck.Path, targetPort, initialDelay, periodSeconds, failureThreshold)
	}

	template := `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
    type: dev-env
spec:
  replicas: 1
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
        app: %s
        type: dev-env
    spec:
      serviceAccountName: %s
      containers:
      - name: dev-environment
        image: %s
        imagePullPolicy: IfNotPresent
        ports:
%s
%s
%s
        resources:
          requests:
            cpu: "%s"
            memory: "%s"
          limits:
            cpu: "%s"
            memory: "%s"
        volumeMounts:
%s
      volumes:
%s`

	return fmt.Sprintf(template,
		req.Name, req.Namespace, req.Name, req.Name, req.Name, req.SAName, req.Image, portsYAML, envVarsYAML, probeYAML,
		req.Resources.CPU, req.Resources.Memory,
		req.Resources.CPULimit, req.Resources.MemoryLimit,
		volumeMountsYAML, volumesYAML)
}

// generateServiceYAML 生成Service的YAML
func generateServiceYAML(req CreateEnvironmentRequest) string {
	// 动态生成端口列表
	portsYAML := ""
	for _, port := range req.Ports {
		portsYAML += fmt.Sprintf(`  - name: %s
    port: %d
    targetPort: %d
    nodePort: %d
    protocol: %s
`, port.Name, port.ServicePort, port.ContainerPort, port.NodePort, port.Protocol)
	}

	template := `---
apiVersion: v1
kind: Service
metadata:
  name: %s-service
  namespace: %s
  labels:
    app: %s
spec:
  type: NodePort
  externalTrafficPolicy: Cluster
  selector:
    app: %s
  ports:
%s`

	return fmt.Sprintf(template,
		req.Name, req.Namespace, req.Name, req.Name, portsYAML)
}

// generateSAKubeconfig 为ServiceAccount生成kubeconfig（不创建权限，权限应该已经存在）
func generateSAKubeconfig(clientset *kubernetes.Clientset, saName, namespace, kubeconfigContent string) (string, error) {
	ctx := context.Background()

	// 解析原始kubeconfig获取集群信息
	originalConfig, err := clientcmd.Load([]byte(kubeconfigContent))
	if err != nil {
		return "", fmt.Errorf("解析原始kubeconfig失败: %v", err)
	}

	// 获取当前上下文
	currentContextName := originalConfig.CurrentContext
	if currentContextName == "" {
		return "", fmt.Errorf("kubeconfig中没有设置当前上下文")
	}

	currentContext := originalConfig.Contexts[currentContextName]
	if currentContext == nil {
		return "", fmt.Errorf("找不到当前上下文: %s", currentContextName)
	}

	// 获取当前集群
	cluster := originalConfig.Clusters[currentContext.Cluster]
	if cluster == nil {
		return "", fmt.Errorf("找不到当前上下文对应的集群: %s", currentContext.Cluster)
	}

	// 验证SA是否存在
	_, err = clientset.CoreV1().ServiceAccounts(namespace).Get(ctx, saName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("ServiceAccount不存在，请先调用权限管理接口创建SA: %v", err)
	}

	// 使用TokenRequest API创建token
	fmt.Printf("🔍 调试信息: 尝试使用TokenRequest API为SA '%s' 创建token\n", saName)
	tokenRequest := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			// 不指定audiences，使用默认值
			ExpirationSeconds: func() *int64 { i := int64(3600); return &i }(), // 1小时过期
		},
	}

	// 创建TokenRequest
	tokenResponse, err := clientset.CoreV1().ServiceAccounts(namespace).CreateToken(ctx, saName, tokenRequest, metav1.CreateOptions{})
	if err != nil {
		fmt.Printf("🔍 调试信息: TokenRequest失败，尝试使用旧的secret方式: %v\n", err)
		// 如果TokenRequest失败，尝试使用旧的secret方式
		var tokenSecret *corev1.Secret
		for i := 0; i < 30; i++ {
			secrets, err := clientset.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return "", fmt.Errorf("获取Secrets失败: %v", err)
			}

			for _, secret := range secrets.Items {
				if strings.HasPrefix(secret.Name, saName+"-token") {
					tokenSecret = &secret
					break
				}
			}

			if tokenSecret != nil {
				break
			}
			time.Sleep(2 * time.Second)
		}

		if tokenSecret == nil {
			return "", fmt.Errorf("未找到ServiceAccount的token secret，且TokenRequest失败: %v", err)
		}

		fmt.Printf("✅ 调试信息: 找到token secret: %s\n", tokenSecret.Name)
		token := tokenSecret.Data["token"]
		if len(token) == 0 {
			return "", fmt.Errorf("token为空")
		}

		fmt.Printf("✅ 调试信息: 从secret获取token成功，token长度: %d\n", len(token))
		return generateKubeconfigFromToken(token, namespace, saName, originalConfig, currentContext.Cluster)
	}

	// 使用TokenRequest获取的token
	if len(tokenResponse.Status.Token) == 0 {
		return "", fmt.Errorf("TokenRequest返回的token为空")
	}

	fmt.Printf("✅ 调试信息: TokenRequest成功，token长度: %d\n", len(tokenResponse.Status.Token))
	return generateKubeconfigFromToken([]byte(tokenResponse.Status.Token), namespace, saName, originalConfig, currentContext.Cluster)
}

// generateKubeconfigFromToken 从token生成kubeconfig
func generateKubeconfigFromToken(token []byte, namespace, saName string, originalConfig *api.Config, clusterName string) (string, error) {
	// 创建kubeconfig的简化版本
	newConfig := api.NewConfig()

	// 使用指定的集群信息
	cluster := originalConfig.Clusters[clusterName]
	if cluster == nil {
		return "", fmt.Errorf("找不到集群: %s", clusterName)
	}

	newConfig.Clusters[clusterName] = &api.Cluster{
		Server:                   cluster.Server,
		CertificateAuthority:     cluster.CertificateAuthority,
		CertificateAuthorityData: cluster.CertificateAuthorityData,
	}

	// 创建用户信息 - 使用标准的ServiceAccount格式
	saAuthName := fmt.Sprintf("system:serviceaccount:%s:%s", namespace, saName)

	// 创建上下文
	kubeContextName := fmt.Sprintf("%s@%s", saAuthName, namespace)
	newConfig.Contexts[kubeContextName] = &api.Context{
		Cluster:   clusterName,
		Namespace: namespace,
		AuthInfo:  saAuthName,
	}

	// 创建用户信息
	newConfig.AuthInfos[saAuthName] = &api.AuthInfo{
		Token: string(token),
	}

	// 设置当前上下文
	newConfig.CurrentContext = kubeContextName

	// 生成kubeconfig YAML
	kubeconfigYAML, err := clientcmd.Write(*newConfig)
	if err != nil {
		return "", fmt.Errorf("生成kubeconfig失败: %v", err)
	}

	return string(kubeconfigYAML), nil
}

// generateKubeconfigFromTokenWithAdmin 从token生成kubeconfig（使用标准ServiceAccount格式）
func generateKubeconfigFromTokenWithAdmin(token []byte, namespace, saName string, originalConfig *api.Config, clusterName string) (string, error) {
	// 创建kubeconfig的简化版本
	newConfig := api.NewConfig()

	// 使用指定的集群信息
	cluster := originalConfig.Clusters[clusterName]
	if cluster == nil {
		return "", fmt.Errorf("找不到集群: %s", clusterName)
	}

	newConfig.Clusters[clusterName] = &api.Cluster{
		Server:                   cluster.Server,
		CertificateAuthority:     cluster.CertificateAuthority,
		CertificateAuthorityData: cluster.CertificateAuthorityData,
	}

	// 创建用户信息 - 使用标准的ServiceAccount格式
	saAuthName := fmt.Sprintf("system:serviceaccount:%s:%s", namespace, saName)

	// 创建上下文
	kubeContextName := fmt.Sprintf("%s@%s", saAuthName, namespace)
	newConfig.Contexts[kubeContextName] = &api.Context{
		Cluster:   clusterName,
		Namespace: namespace,
		AuthInfo:  saAuthName,
	}

	// 创建用户信息
	newConfig.AuthInfos[saAuthName] = &api.AuthInfo{
		Token: string(token),
	}

	// 设置当前上下文
	newConfig.CurrentContext = kubeContextName

	// 生成kubeconfig YAML
	kubeconfigYAML, err := clientcmd.Write(*newConfig)
	if err != nil {
		return "", fmt.Errorf("生成kubeconfig失败: %v", err)
	}

	return string(kubeconfigYAML), nil
}

// generateSAKubeconfigWithAdmin 使用管理员权限为ServiceAccount生成kubeconfig
func generateSAKubeconfigWithAdmin(clientset *kubernetes.Clientset, saName, namespace, kubeconfigContent string, expirationHours int64) (string, error) {
	ctx := context.Background()

	// 解析原始kubeconfig获取集群信息
	originalConfig, err := clientcmd.Load([]byte(kubeconfigContent))
	if err != nil {
		return "", fmt.Errorf("解析原始kubeconfig失败: %v", err)
	}

	// 获取当前上下文
	currentContextName := originalConfig.CurrentContext
	if currentContextName == "" {
		return "", fmt.Errorf("kubeconfig中没有设置当前上下文")
	}

	currentContext := originalConfig.Contexts[currentContextName]
	if currentContext == nil {
		return "", fmt.Errorf("找不到当前上下文: %s", currentContextName)
	}

	// 获取当前集群
	cluster := originalConfig.Clusters[currentContext.Cluster]
	if cluster == nil {
		return "", fmt.Errorf("找不到当前上下文对应的集群: %s", currentContext.Cluster)
	}

	// 验证SA是否存在
	_, err = clientset.CoreV1().ServiceAccounts(namespace).Get(ctx, saName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("ServiceAccount不存在，请先调用权限管理接口创建SA: %v", err)
	}

	// 使用管理员权限创建SA的token
	fmt.Printf("🔍 调试信息: 使用管理员权限为SA '%s' 创建token\n", saName)

	// 使用传入的过期时间
	expirationSeconds := expirationHours * 3600 // 转换为秒
	fmt.Printf("🔍 调试信息: Token过期时间设置为 %d 秒 (%.1f 天)\n", expirationSeconds, float64(expirationSeconds)/(24*3600))

	tokenRequest := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			// 不指定audiences，使用默认值
			ExpirationSeconds: &expirationSeconds,
		},
	}

	// 创建TokenRequest（使用管理员权限）
	tokenResponse, err := clientset.CoreV1().ServiceAccounts(namespace).CreateToken(ctx, saName, tokenRequest, metav1.CreateOptions{})
	if err != nil {
		fmt.Printf("🔍 调试信息: 管理员TokenRequest失败，尝试使用旧的secret方式: %v\n", err)
		// 如果TokenRequest失败，尝试使用旧的secret方式
		var tokenSecret *corev1.Secret
		for i := 0; i < 30; i++ {
			secrets, err := clientset.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return "", fmt.Errorf("获取Secrets失败: %v", err)
			}

			for _, secret := range secrets.Items {
				if strings.HasPrefix(secret.Name, saName+"-token") {
					tokenSecret = &secret
					break
				}
			}

			if tokenSecret != nil {
				break
			}
			time.Sleep(2 * time.Second)
		}

		if tokenSecret == nil {
			return "", fmt.Errorf("未找到ServiceAccount的token secret，且TokenRequest失败: %v", err)
		}

		fmt.Printf("✅ 调试信息: 找到token secret: %s\n", tokenSecret.Name)
		token := tokenSecret.Data["token"]
		if len(token) == 0 {
			return "", fmt.Errorf("token为空")
		}

		fmt.Printf("✅ 调试信息: 从secret获取token成功，token长度: %d\n", len(token))
		return generateKubeconfigFromToken(token, namespace, saName, originalConfig, currentContext.Cluster)
	}

	// 使用管理员TokenRequest获取的token
	if len(tokenResponse.Status.Token) == 0 {
		return "", fmt.Errorf("TokenRequest返回的token为空")
	}

	fmt.Printf("✅ 调试信息: 管理员TokenRequest成功，token长度: %d\n", len(tokenResponse.Status.Token))
	return generateKubeconfigFromTokenWithAdmin([]byte(tokenResponse.Status.Token), namespace, saName, originalConfig, currentContext.Cluster)
}

// createSAPermissions 创建ServiceAccount的权限（Role和RoleBinding）
func createSAPermissions(ctx context.Context, clientset *kubernetes.Clientset, saName, namespace string) error {
	// 创建Role（如果不存在）
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-full-access", saName),
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
		},
	}

	_, err := clientset.RbacV1().Roles(namespace).Create(ctx, role, metav1.CreateOptions{})
	if err != nil && !isAlreadyExistsError(err) {
		return fmt.Errorf("创建Role失败: %v", err)
	}

	// 创建RoleBinding
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-binding", saName),
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     fmt.Sprintf("%s-full-access", saName),
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	_, err = clientset.RbacV1().RoleBindings(namespace).Create(ctx, roleBinding, metav1.CreateOptions{})
	if err != nil && !isAlreadyExistsError(err) {
		return fmt.Errorf("创建RoleBinding失败: %v", err)
	}

	return nil
}

// isAlreadyExistsError 检查是否为资源已存在错误
func isAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	// 检查是否包含"already exists"关键字，适用于各种Kubernetes资源类型
	return strings.Contains(errMsg, "already exists")
}

// isNotFoundError 检查是否为资源未找到错误
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	// 检查是否包含"not found"关键字，适用于各种Kubernetes资源类型
	return strings.Contains(errMsg, "not found")
}

// ResourceLimits 资源限制配置（从请求参数传入）
type ResourceLimits struct {
	CPU      string
	Memory   string
	Storage  string
	PodCount string
}

// createResourceQuota 创建ResourceQuota
func createResourceQuota(ctx context.Context, clientset *kubernetes.Clientset, namespace string, limits ResourceLimits) error {
	// 设置默认值
	if limits.CPU == "" {
		limits.CPU = "8"
	}
	if limits.Memory == "" {
		limits.Memory = "16Gi"
	}
	if limits.Storage == "" {
		limits.Storage = "20Gi"
	}
	if limits.PodCount == "" {
		limits.PodCount = "2"
	}

	// 创建ResourceQuota对象
	resourceQuota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "resource-quota",
			Namespace: namespace,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{},
		},
	}

	// 设置CPU限制
	if limits.CPU != "" {
		resourceQuota.Spec.Hard[corev1.ResourceLimitsCPU] = resource.MustParse(limits.CPU)
		resourceQuota.Spec.Hard[corev1.ResourceRequestsCPU] = resource.MustParse(limits.CPU)
	}

	// 设置内存限制
	if limits.Memory != "" {
		resourceQuota.Spec.Hard[corev1.ResourceLimitsMemory] = resource.MustParse(limits.Memory)
		resourceQuota.Spec.Hard[corev1.ResourceRequestsMemory] = resource.MustParse(limits.Memory)
	}

	// 设置存储限制
	if limits.Storage != "" {
		resourceQuota.Spec.Hard["requests.storage"] = resource.MustParse(limits.Storage)
	}

	// 设置Pod数量限制
	if limits.PodCount != "" {
		resourceQuota.Spec.Hard[corev1.ResourcePods] = resource.MustParse(limits.PodCount)
	}

	// 尝试创建ResourceQuota
	_, err := clientset.CoreV1().ResourceQuotas(namespace).Create(ctx, resourceQuota, metav1.CreateOptions{})
	if err != nil {
		if isAlreadyExistsError(err) {
			// ResourceQuota已存在，更新它
			existing, err := clientset.CoreV1().ResourceQuotas(namespace).Get(ctx, "resource-quota", metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("获取现有ResourceQuota失败: %v", err)
			}

			// 更新Hard限制
			existing.Spec.Hard = resourceQuota.Spec.Hard

			// 更新ResourceQuota
			_, err = clientset.CoreV1().ResourceQuotas(namespace).Update(ctx, existing, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("更新ResourceQuota失败: %v", err)
			}
			fmt.Printf("✅ ResourceQuota已更新 (pods: %s, cpu: %s, memory: %s, storage: %s)\n",
				limits.PodCount, limits.CPU, limits.Memory, limits.Storage)
		} else {
			return fmt.Errorf("创建ResourceQuota失败: %v", err)
		}
	} else {
		fmt.Printf("✅ ResourceQuota已创建 (pods: %s, cpu: %s, memory: %s, storage: %s)\n",
			limits.PodCount, limits.CPU, limits.Memory, limits.Storage)
	}

	return nil
}

// getAdminKubeconfig 获取挂载的管理员kubeconfig
func getAdminKubeconfig() (string, error) {
	adminKubeconfigPath := "/app/config/admin-kubeconfig"

	// 检查文件是否存在
	if _, err := os.Stat(adminKubeconfigPath); os.IsNotExist(err) {
		return "", fmt.Errorf("管理员kubeconfig文件不存在: %s", adminKubeconfigPath)
	}

	// 读取文件内容
	content, err := os.ReadFile(adminKubeconfigPath)
	if err != nil {
		return "", fmt.Errorf("读取管理员kubeconfig文件失败: %v", err)
	}

	return string(content), nil
}

// buildKubeconfigFromInCluster 从InCluster配置构建kubeconfig内容
// 用于在Pod内部运行时生成SA kubeconfig
func buildKubeconfigFromInCluster() (string, error) {
	inClusterConfig, err := rest.InClusterConfig()
	if err != nil {
		return "", fmt.Errorf("获取InCluster配置失败: %v", err)
	}

	// 读取CA证书
	caData, err := os.ReadFile(inClusterConfig.TLSClientConfig.CAFile)
	if err != nil {
		return "", fmt.Errorf("读取CA证书失败: %v", err)
	}

	// 构建kubeconfig
	config := api.NewConfig()
	clusterName := "default-cluster"
	userName := "in-cluster-user"
	contextName := "in-cluster-context"

	// 设置集群信息
	config.Clusters[clusterName] = &api.Cluster{
		Server:                   inClusterConfig.Host,
		CertificateAuthorityData: caData,
	}

	// 设置用户信息（使用token）
	config.AuthInfos[userName] = &api.AuthInfo{
		Token: inClusterConfig.BearerToken,
	}

	// 设置上下文
	config.Contexts[contextName] = &api.Context{
		Cluster:  clusterName,
		AuthInfo: userName,
	}

	// 设置当前上下文
	config.CurrentContext = contextName

	// 生成YAML
	kubeconfigYAML, err := clientcmd.Write(*config)
	if err != nil {
		return "", fmt.Errorf("生成kubeconfig失败: %v", err)
	}

	return string(kubeconfigYAML), nil
}

// resolveKubeconfig 解析kubeconfig，优先级：
// 1. 用户传入的kubeconfig
// 2. InCluster配置（Pod内部运行时）
// 3. 挂载的管理员kubeconfig文件
func resolveKubeconfig(userKubeconfig string) (string, error) {
	// 优先级1: 用户传入的kubeconfig
	if userKubeconfig != "" {
		fmt.Printf("🔍 调试信息: 使用用户传入的kubeconfig\n")
		return userKubeconfig, nil
	}

	// 优先级2: 尝试InCluster配置
	inClusterKubeconfig, err := buildKubeconfigFromInCluster()
	if err == nil {
		fmt.Printf("🔍 调试信息: 使用InCluster配置\n")
		return inClusterKubeconfig, nil
	}
	fmt.Printf("🔍 调试信息: InCluster配置不可用 (%v)，尝试挂载的kubeconfig\n", err)

	// 优先级3: 使用挂载的管理员kubeconfig
	adminKubeconfig, err := getAdminKubeconfig()
	if err != nil {
		return "", fmt.Errorf("无法获取kubeconfig: InCluster不可用且文件挂载失败")
	}

	fmt.Printf("🔍 调试信息: 使用挂载的管理员kubeconfig\n")
	return adminKubeconfig, nil
}

// analyzeError 分析错误类型
func analyzeError(err error) (isNormal bool, errorType string) {
	if err == nil {
		return true, "success"
	}

	errMsg := err.Error()

	// 权限错误 - 应该返回失败
	if strings.Contains(errMsg, "is forbidden") ||
		strings.Contains(errMsg, "Unauthorized") ||
		strings.Contains(errMsg, "permission denied") {
		return false, "permission"
	}

	// 资源不存在 - 这是正常情况，应该返回成功
	if strings.Contains(errMsg, "not found") {
		return true, "not_found"
	}

	// 认证错误 - 应该返回失败
	if strings.Contains(errMsg, "unauthorized") ||
		strings.Contains(errMsg, "invalid token") ||
		strings.Contains(errMsg, "authentication failed") {
		return false, "auth"
	}

	// 网络或API错误 - 应该返回失败
	if strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "network") ||
		strings.Contains(errMsg, "internal server error") {
		return false, "network"
	}

	// 其他未知错误 - 应该返回失败
	return false, "unknown"
}

// isPermissionError 检查是否为权限错误（保留用于兼容）
func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "is forbidden") ||
		strings.Contains(errMsg, "Unauthorized") ||
		strings.Contains(errMsg, "permission denied")
}