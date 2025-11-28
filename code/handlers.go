package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	sigsyaml "sigs.k8s.io/yaml"
)

// CreateEnvironmentRequest 创建环境请求
type CreateEnvironmentRequest struct {
	Kubeconfig string `json:"kubeconfig" binding:"required" example:"apiVersion: v1\nkind: Config\n..."`
	Name       string `json:"name" binding:"required" example:"demo"`
	Namespace  string `json:"namespace" example:"dev-space" note:"可选，默认使用kubeconfig中的namespace"`
	// 以下字段由系统从kubeconfig中解析，不需要用户传入
	SAName     string `json:"-"` // 从kubeconfig中提取的ServiceAccount名称
	Resources  struct {
		CPU        string `json:"cpu" binding:"required" example:"1000m"`
		CPULimit   string `json:"cpu_limit" binding:"required" example:"2000m"`
		Memory     string `json:"memory" binding:"required" example:"2Gi"`
		MemoryLimit string `json:"memory_limit" binding:"required" example:"4Gi"`
	} `json:"resources" binding:"required"`
	Storage struct {
		Workspace string `json:"workspace" binding:"required" example:"10Gi"`
		VSCode    string `json:"vscode" binding:"required" example:"5Gi"`
	} `json:"storage" binding:"required"`
	NodePorts struct {
		VSCode   int `json:"vscode" example:"0"`
		SSH      int `json:"ssh" example:"0"`
		Terminal int `json:"terminal" example:"0"`
	} `json:"nodeports"`
}

// KubeconfigRequest 创建ServiceAccount请求参数
type KubeconfigRequest struct {
	Kubeconfig        string `json:"kubeconfig" example:"apiVersion: v1\nkind: Config\n..." note:"可选，不传入则使用挂载的管理员kubeconfig"`
	Namespace         string `json:"namespace" example:"default"`
	CreateIfNotExists bool   `json:"create_if_not_exists" example:"true"`
	TokenExpiration   int64  `json:"token_expiration_hours" example:"1" note:"Token过期时间（小时），默认：1小时"`
	ResourceLimits    struct {
		CPU       string `json:"cpu" example:"8" note:"CPU限制，默认：8，例如：8、4000m"`
		Memory    string `json:"memory" example:"16Gi" note:"内存限制，默认：16Gi，例如：16Gi、16000Mi"`
		Storage   string `json:"storage" example:"20Gi" note:"存储限制，默认：20Gi，例如：20Gi"`
		PodCount  string `json:"pod_count" example:"2" note:"Pod数量限制，默认：2，例如：2"`
	} `json:"resource_limits,omitempty" note:"资源限制配置，字段不填时使用默认值"`
}

// DeleteEnvironmentRequest 删除环境请求参数
type DeleteEnvironmentRequest struct {
	Kubeconfig string `json:"kubeconfig" binding:"required" example:"apiVersion: v1\nkind: Config\n..."`
	Namespace  string `json:"namespace" binding:"required" example:"dev-space"`
}

// GetEnvironmentRequest 获取环境信息请求参数
type GetEnvironmentRequest struct {
	Kubeconfig string `json:"kubeconfig" binding:"required" example:"apiVersion: v1\nkind: Config\n..."`
	Namespace  string `json:"namespace" binding:"required" example:"dev-space"`
}

// DeleteServiceAccountRequest 删除ServiceAccount请求参数
type DeleteServiceAccountRequest struct {
	Kubeconfig string `json:"kubeconfig" example:"apiVersion: v1\nkind: Config\n..." note:"可选，不传入则使用挂载的管理员kubeconfig"`
	Namespace  string `json:"namespace" binding:"required" example:"dev-space"`
}

// APIResponse 通用API响应
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// 从kubeconfig内容创建k8s客户端
func createK8sClientFromKubeconfig(kubeconfigContent string) (*kubernetes.Clientset, error) {
	fmt.Printf("🔍 调试信息: createK8sClientFromKubeconfig - 开始解析kubeconfig")

	// 解析kubeconfig内容
	config, err := clientcmd.Load([]byte(kubeconfigContent))
	if err != nil {
		fmt.Printf("🔍 调试信息: kubeconfig解析失败: %v\n", err)
		return nil, fmt.Errorf("解析kubeconfig失败: %v", err)
	}
	fmt.Printf("🔍 调试信息: kubeconfig解析成功\n")

	// 创建REST配置
	restConfig, err := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		fmt.Printf("🔍 调试信息: REST配置创建失败: %v\n", err)
		return nil, fmt.Errorf("创建REST配置失败: %v", err)
	}

	// 创建clientset
	fmt.Printf("🔍 调试信息: 开始创建k8s客户端...")
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		fmt.Printf("🔍 调试信息: k8s客户端创建失败: %v\n", err)
		return nil, fmt.Errorf("创建k8s客户端失败: %v", err)
	}

	fmt.Printf("🔍 调试信息: k8s客户端创建成功\n")
	return clientset, nil
}

// CreateEnvironment 创建环境
// @Summary 创建环境
// @Description 基于ServiceAccount的kubeconfig创建完整的K8s环境，包括PVC、Deployment和Service。使用传入的kubeconfig中的身份信息
// @Tags environments
// @Accept json
// @Produce json
// @Param request body CreateEnvironmentRequest true "创建环境请求"
// @Success 200 {object} APIResponse{data=string} "成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /environments [post]
func CreateEnvironment(c *gin.Context) {
	var req CreateEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	// 创建k8s客户端（使用传入的SA kubeconfig）
	fmt.Printf("🔍 调试信息: 开始创建k8s客户端...")
	clientset, err := createK8sClientFromKubeconfig(req.Kubeconfig)
	if err != nil {
		fmt.Printf("🔍 调试信息: k8s客户端创建失败: %v\n", err)
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "创建k8s客户端失败: " + err.Error(),
		})
		return
	}
	fmt.Printf("🔍 调试信息: k8s客户端创建成功\n")

	// 从kubeconfig中获取当前上下文信息
	fmt.Printf("🔍 调试信息: 开始解析kubeconfig (长度: %d 字符)\n", len(req.Kubeconfig))

	config, err := clientcmd.Load([]byte(req.Kubeconfig))
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "解析kubeconfig失败: " + err.Error(),
		})
		return
	}

	// 获取当前上下文
	currentContextName := config.CurrentContext
	fmt.Printf("🔍 调试信息: 当前上下文='%s'\n", currentContextName)

	if currentContextName == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "kubeconfig中没有设置当前上下文",
		})
		return
	}

	currentContext := config.Contexts[currentContextName]
	if currentContext == nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "找不到当前上下文: " + currentContextName,
		})
		return
	}

	// 获取ServiceAccount名称和namespace
	saName := currentContext.AuthInfo
	namespace := req.Namespace
	if namespace == "" {
		namespace = currentContext.Namespace
	}

	fmt.Printf("🔍 调试信息: 从kubeconfig解析 - AuthInfo='%s', ContextNamespace='%s', RequestNamespace='%s'\n",
		saName, currentContext.Namespace, req.Namespace)

	// 验证SA名称格式（如果是ServiceAccount）
	var actualSAName string
	fmt.Printf("🔍 调试信息: 解析SA名称 - 原始AuthInfo='%s'\n", saName)

	if strings.HasPrefix(saName, "system:serviceaccount:") {
		// 格式：system:serviceaccount:namespace:sa-name
		parts := strings.Split(saName, ":")
		fmt.Printf("🔍 调试信息: ServiceAccount格式解析 - parts=%v\n", parts)
		if len(parts) == 4 {
			actualSAName = parts[3]
			if namespace == "" {
				namespace = parts[2]
			}
			fmt.Printf("🔍 调试信息: 解析结果 - SA='%s', 来自kubeconfig的namespace='%s'\n", actualSAName, parts[2])
		} else {
			fmt.Printf("🔍 调试信息: ServiceAccount格式异常，期望4个部分，实际%d个\n", len(parts))
		}
	} else {
		actualSAName = saName
		fmt.Printf("🔍 调试信息: 非标准ServiceAccount格式，直接使用='%s'\n", actualSAName)
	}

	if actualSAName == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "无法从kubeconfig中确定ServiceAccount名称",
		})
		return
	}

	if namespace == "" {
		namespace = "default"
		fmt.Printf("🔍 调试信息: namespace为空，设置为默认值='%s'\n", namespace)
	}

	fmt.Printf("🔍 调试信息: 最终确定 - SA='%s', namespace='%s'\n", actualSAName, namespace)

	ctx := context.Background()

	fmt.Printf("✅ 调试信息: 使用ServiceAccount '%s' 在namespace '%s' 中创建环境\n", actualSAName, namespace)

	// 设置SA名称和namespace到请求中
	req.SAName = actualSAName
	req.Namespace = namespace

	// 设置NodePorts默认值（如果是零值）
	setNodePortsDefaults(&req)

	// 创建PVC
	err = createPVCs(ctx, clientset, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "创建PVC失败: " + err.Error(),
		})
		return
	}

	// 创建Deployment
	err = createDeployment(ctx, clientset, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "创建Deployment失败: " + err.Error(),
		})
		return
	}

	// 创建Service
	err = createService(ctx, clientset, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "创建Service失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "开发环境部署成功（支持幂等操作）",
		Data: gin.H{
			"namespace":    req.Namespace,
			"service_name": fmt.Sprintf("%s-service", req.Name),
			"sa_name":      req.SAName,
		},
	})
}

// CreateServiceAccount 创建ServiceAccount并返回kubeconfig
// @Summary 创建ServiceAccount并返回kubeconfig
// @Description 创建指定的ServiceAccount，分配权限，并返回对应的kubeconfig
// @Tags service-accounts
// @Accept json
// @Produce json
// @Param name path string true "ServiceAccount名称"
// @Param request body KubeconfigRequest true "创建ServiceAccount请求参数"
// @Success 200 {object} APIResponse{data=string} "成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /service-accounts/{name} [post]
func CreateServiceAccount(c *gin.Context) {
	var req KubeconfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	saName := c.Param("name")
	if saName == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "ServiceAccount名称不能为空",
		})
		return
	}

	// 设置默认namespace
	if req.Namespace == "" {
		req.Namespace = "default"
	}

	// 设置默认Token过期时间（1小时）
	if req.TokenExpiration == 0 {
		req.TokenExpiration = 1 // 默认1小时
		fmt.Printf("🔍 调试信息: Token过期时间为空，使用默认值: %d 小时\n", req.TokenExpiration)
	}

	// 解析kubeconfig（优先使用用户传入的，否则使用管理员kubeconfig）
	kubeconfig, err := resolveKubeconfig(req.Kubeconfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "解析kubeconfig失败: " + err.Error(),
		})
		return
	}

	// 创建k8s客户端
	clientset, err := createK8sClientFromKubeconfig(kubeconfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "创建k8s客户端失败: " + err.Error(),
		})
		return
	}

	ctx := context.Background()

	// 检查SA是否存在，如果不存在则创建
	if req.CreateIfNotExists {
		// 先检查namespace是否存在，如果不存在则创建
		_, err = clientset.CoreV1().Namespaces().Get(ctx, req.Namespace, metav1.GetOptions{})
		if err != nil {
			// 如果namespace不存在，创建namespace
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: req.Namespace,
				},
			}
			_, err = clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
			if err != nil {
				c.JSON(http.StatusInternalServerError, APIResponse{
					Success: false,
					Message: "创建namespace失败: " + err.Error(),
				})
				return
			}
		}

		// 检查SA是否存在
		_, err = clientset.CoreV1().ServiceAccounts(req.Namespace).Get(ctx, saName, metav1.GetOptions{})
		if err != nil {
			// 如果SA不存在，创建SA
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      saName,
					Namespace: req.Namespace,
				},
			}
			_, err = clientset.CoreV1().ServiceAccounts(req.Namespace).Create(ctx, sa, metav1.CreateOptions{})
			if err != nil {
				c.JSON(http.StatusInternalServerError, APIResponse{
					Success: false,
					Message: "创建ServiceAccount失败: " + err.Error(),
				})
				return
			}
		}

		// 创建ResourceQuota资源配额
		resourceLimits := ResourceLimits{
			CPU:      req.ResourceLimits.CPU,
			Memory:   req.ResourceLimits.Memory,
			Storage:  req.ResourceLimits.Storage,
			PodCount: req.ResourceLimits.PodCount,
		}
		err = createResourceQuota(ctx, clientset, req.Namespace, resourceLimits)
		if err != nil {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Success: false,
				Message: "创建ResourceQuota失败: " + err.Error(),
			})
			return
		}
	}

	// 创建SA的权限（Role和RoleBinding）
	fmt.Printf("🔍 调试信息: 开始为SA '%s' 创建权限\n", saName)
	err = createSAPermissions(ctx, clientset, saName, req.Namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "创建SA权限失败: " + err.Error(),
		})
		return
	}
	fmt.Printf("✅ 调试信息: SA '%s' 权限创建成功\n", saName)

	// 生成SA的kubeconfig（使用传入的管理员权限来创建Token，而不是SA自己的权限）
	fmt.Printf("🔍 调试信息: 开始为SA '%s' 生成kubeconfig，Token过期时间: %d 小时\n", saName, req.TokenExpiration)
	saKubeconfig, err := generateSAKubeconfigWithAdmin(clientset, saName, req.Namespace, kubeconfig, req.TokenExpiration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "生成kubeconfig失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "kubeconfig生成成功",
		Data:    saKubeconfig,
	})
}

// DeleteServiceAccount 删除ServiceAccount
// @Summary 删除ServiceAccount
// @Description 删除指定的ServiceAccount及其相关权限配置（Role、RoleBinding、ResourceQuota）
// @Tags service-accounts
// @Accept json
// @Produce json
// @Param name path string true "ServiceAccount名称"
// @Param request body DeleteServiceAccountRequest true "删除ServiceAccount请求参数"
// @Success 200 {object} APIResponse "成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /service-accounts/{name} [delete]
func DeleteServiceAccount(c *gin.Context) {
	var req DeleteServiceAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	saName := c.Param("name")
	if saName == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "ServiceAccount名称不能为空",
		})
		return
	}

	// 解析kubeconfig（优先使用用户传入的，否则使用管理员kubeconfig）
	kubeconfig, err := resolveKubeconfig(req.Kubeconfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "解析kubeconfig失败: " + err.Error(),
		})
		return
	}

	// 创建k8s客户端
	clientset, err := createK8sClientFromKubeconfig(kubeconfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "创建k8s客户端失败: " + err.Error(),
		})
		return
	}

	ctx := context.Background()

	// 删除RoleBinding
	roleBindingName := fmt.Sprintf("%s-binding", saName)
	err = clientset.RbacV1().RoleBindings(req.Namespace).Delete(ctx, roleBindingName, metav1.DeleteOptions{})
	if err != nil && !isNotFoundError(err) {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "删除RoleBinding失败: " + err.Error(),
		})
		return
	}

	// 删除Role
	roleName := fmt.Sprintf("%s-full-access", saName)
	err = clientset.RbacV1().Roles(req.Namespace).Delete(ctx, roleName, metav1.DeleteOptions{})
	if err != nil && !isNotFoundError(err) {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "删除Role失败: " + err.Error(),
		})
		return
	}

	// 删除ResourceQuota（只有当namespace中只有一个ResourceQuota时才删除）
	resourceQuotaName := "resource-quota"
	err = clientset.CoreV1().ResourceQuotas(req.Namespace).Delete(ctx, resourceQuotaName, metav1.DeleteOptions{})
	if err != nil && !isNotFoundError(err) {
		// ResourceQuota删除失败不影响整体操作，只记录日志
		fmt.Printf("警告: 删除ResourceQuota失败: %v\n", err)
	}

	// 删除ServiceAccount
	err = clientset.CoreV1().ServiceAccounts(req.Namespace).Delete(ctx, saName, metav1.DeleteOptions{})
	if err != nil && !isNotFoundError(err) {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "删除ServiceAccount失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "ServiceAccount删除成功",
		Data: gin.H{
			"service_account": saName,
			"namespace":       req.Namespace,
		},
	})
}

// DeleteEnvironment 删除环境
// @Summary 删除环境
// @Description 删除指定名称的环境，包括PVC、Deployment和Service
// @Tags environments
// @Accept json
// @Produce json
// @Param name path string true "环境名称"
// @Param request body DeleteEnvironmentRequest true "删除环境请求参数"
// @Success 200 {object} APIResponse "成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /environments/{name} [delete]
func DeleteEnvironment(c *gin.Context) {
	var req DeleteEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	envName := c.Param("name")
	if envName == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "环境名称不能为空",
		})
		return
	}

	// 创建k8s客户端
	clientset, err := createK8sClientFromKubeconfig(req.Kubeconfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "创建k8s客户端失败: " + err.Error(),
		})
		return
	}

	ctx := context.Background()

	// 删除Service
	err = clientset.CoreV1().Services(req.Namespace).Delete(ctx, envName+"-service", metav1.DeleteOptions{})
	if err != nil && !isNotFoundError(err) {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "删除Service失败: " + err.Error(),
		})
		return
	}

	// 删除Deployment
	err = clientset.AppsV1().Deployments(req.Namespace).Delete(ctx, envName, metav1.DeleteOptions{})
	if err != nil && !isNotFoundError(err) {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "删除Deployment失败: " + err.Error(),
		})
		return
	}

	// 删除PVCs
	pvcs := []string{envName + "-workspace", envName + "-vscode"}
	for _, pvcName := range pvcs {
		err = clientset.CoreV1().PersistentVolumeClaims(req.Namespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
		if err != nil && !isNotFoundError(err) {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Success: false,
				Message: fmt.Sprintf("删除PVC %s 失败: %v", pvcName, err),
			})
			return
		}
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "环境删除成功",
	})
}

// GetEnvironment 获取环境信息
// @Summary 获取环境信息
// @Description 获取指定名称的环境的状态信息
// @Tags environments
// @Accept json
// @Produce json
// @Param name path string true "环境名称"
// @Param request body GetEnvironmentRequest true "获取环境信息请求参数"
// @Success 200 {object} APIResponse{data=map[string]interface{}} "成功"
// @Failure 400 {object} APIResponse "请求参数错误"
// @Failure 500 {object} APIResponse "服务器内部错误"
// @Router /environments/{name} [post]
func GetEnvironment(c *gin.Context) {
	var req GetEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "请求参数错误: " + err.Error(),
		})
		return
	}

	envName := c.Param("name")
	if envName == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "环境名称不能为空",
		})
		return
	}

	// 创建k8s客户端
	clientset, err := createK8sClientFromKubeconfig(req.Kubeconfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Message: "创建k8s客户端失败: " + err.Error(),
		})
		return
	}

	ctx := context.Background()

	envStatus := make(map[string]interface{})
	criticalErrors := []string{} // 关键错误，应该返回失败
	permissionErrors := []string{} // 权限错误，应该返回失败

	// 检查Deployment状态
	deployment, err := clientset.AppsV1().Deployments(req.Namespace).Get(ctx, envName, metav1.GetOptions{})
	if err != nil {
		isNormal, errorType := analyzeError(err)
		if !isNormal {
			if errorType == "permission" {
				permissionErrors = append(permissionErrors, fmt.Sprintf("Deployment访问权限不足: %v", err))
			} else {
				criticalErrors = append(criticalErrors, fmt.Sprintf("Deployment访问失败(%s): %v", errorType, err))
			}
		}
		envStatus["deployment"] = map[string]interface{}{
			"exists":    false,
			"error":     err.Error(),
			"errorType": errorType,
		}
	} else {
		envStatus["deployment"] = map[string]interface{}{
			"exists":   true,
			"replicas": deployment.Status.ReadyReplicas,
			"ready":    deployment.Status.ReadyReplicas == *deployment.Spec.Replicas,
		}
	}

	// 检查Service状态
	service, err := clientset.CoreV1().Services(req.Namespace).Get(ctx, envName+"-service", metav1.GetOptions{})
	if err != nil {
		isNormal, errorType := analyzeError(err)
		if !isNormal {
			if errorType == "permission" {
				permissionErrors = append(permissionErrors, fmt.Sprintf("Service访问权限不足: %v", err))
			} else {
				criticalErrors = append(criticalErrors, fmt.Sprintf("Service访问失败(%s): %v", errorType, err))
			}
		}
		envStatus["service"] = map[string]interface{}{
			"exists":    false,
			"error":     err.Error(),
			"errorType": errorType,
		}
	} else {
		serviceInfo := map[string]interface{}{
			"exists":    true,
			"type":      service.Spec.Type,
			"clusterIP": service.Spec.ClusterIP,
		}

		// 如果是NodePort类型，提取NodePort端口信息
		if service.Spec.Type == corev1.ServiceTypeNodePort {
			nodePorts := make(map[string]int)
			for _, port := range service.Spec.Ports {
				if port.NodePort != 0 {
					nodePorts[port.Name] = int(port.NodePort)
				}
			}
			serviceInfo["nodePorts"] = nodePorts
		}

		envStatus["service"] = serviceInfo
	}

	// 检查PVC状态
	pvcs := []string{envName + "-workspace", envName + "-vscode"}
	pvcStatus := make(map[string]interface{})
	for _, pvcName := range pvcs {
		pvc, err := clientset.CoreV1().PersistentVolumeClaims(req.Namespace).Get(ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			isNormal, errorType := analyzeError(err)
			if !isNormal {
				if errorType == "permission" {
					permissionErrors = append(permissionErrors, fmt.Sprintf("PVC '%s' 访问权限不足: %v", pvcName, err))
				} else {
					criticalErrors = append(criticalErrors, fmt.Sprintf("PVC '%s' 访问失败(%s): %v", pvcName, errorType, err))
				}
			}
			pvcStatus[pvcName] = map[string]interface{}{
				"exists":    false,
				"error":     err.Error(),
				"errorType": errorType,
			}
		} else {
			pvcStatus[pvcName] = map[string]interface{}{
				"exists":  true,
				"status":  string(pvc.Status.Phase),
				"storage": pvc.Spec.Resources.Requests.Storage().String(),
			}
		}
	}
	envStatus["pvcs"] = pvcStatus

	// 添加整体环境状态
	deploymentReady := false
	serviceExists := false
	if deployment, ok := envStatus["deployment"].(map[string]interface{}); ok {
		if ready, ok := deployment["ready"].(bool); ok {
			deploymentReady = ready
		}
	}
	if service, ok := envStatus["service"].(map[string]interface{}); ok {
		if exists, ok := service["exists"].(bool); ok {
			serviceExists = exists
		}
	}

	// 计算整体状态
	var overallStatus string
	switch {
	case !deploymentReady && !serviceExists:
		overallStatus = "Creating"
	case deploymentReady && serviceExists:
		overallStatus = "Running"
	case deploymentReady && !serviceExists:
		overallStatus = "Partial"
	default:
		overallStatus = "Pending"
	}

	envStatus["overall_status"] = overallStatus
	envStatus["environment_name"] = envName
	envStatus["namespace"] = req.Namespace

	// 添加访问信息
	accessInfo := make(map[string]interface{})
	passwordInfo := make(map[string]string)

	if service, ok := envStatus["service"].(map[string]interface{}); ok && service["exists"].(bool) {
		if nodePorts, ok := service["nodePorts"].(map[string]int); ok {
			// 从环境变量获取 node-ip
			nodeIP := os.Getenv("NODE_IP")
			if nodeIP == "" {
				nodeIP = "<node-ip>"
			}

			// 尝试从 Pod 环境变量获取密码
			podList, err := clientset.CoreV1().Pods(req.Namespace).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("app=%s", envName),
			})

			if err == nil && len(podList.Items) > 0 {
				pod := podList.Items[0] // 获取第一个 Pod
				for _, env := range pod.Spec.Containers[0].Env {
					switch env.Name {
					case "ACCESS_PASSWORD":
						passwordInfo["vscode"] = env.Value
					case "ROOT_PASSWORD":
						passwordInfo["ssh"] = env.Value
					case "TTYD_PASSWORD":
						passwordInfo["terminal"] = env.Value
					}
				}
			}

			accessURLs := make(map[string]interface{})
			for portName, nodePort := range nodePorts {
				switch portName {
				case "vscode-web":
					vscodeInfo := map[string]interface{}{
						"url": fmt.Sprintf("http://%s:%d", nodeIP, nodePort),
					}
					if password, ok := passwordInfo["vscode"]; ok {
						vscodeInfo["password"] = password
					}
					accessURLs["vscode"] = vscodeInfo
				case "ssh":
					sshInfo := map[string]interface{}{
						"command": fmt.Sprintf("ssh root@%s -p %d", nodeIP, nodePort),
					}
					if password, ok := passwordInfo["ssh"]; ok {
						sshInfo["password"] = password
					}
					accessURLs["ssh"] = sshInfo
				case "web-terminal":
					terminalInfo := map[string]interface{}{
						"url": fmt.Sprintf("http://%s:%d", nodeIP, nodePort),
					}
					if password, ok := passwordInfo["terminal"]; ok {
						terminalInfo["password"] = password // 使用专门的 TTYD_PASSWORD
					}
					// 如果没有 TTYD_PASSWORD，就不设置 password 字段，表示无密码
					accessURLs["terminal"] = terminalInfo
				}
			}
			accessInfo["urls"] = accessURLs
			if nodeIP == "<node-ip>" {
				accessInfo["note"] = "请设置 NODE_IP 环境变量或手动替换 <node-ip> 为实际的服务器IP地址"
			} else {
				accessInfo["note"] = fmt.Sprintf("使用服务器IP: %s", nodeIP)
			}

			// 添加默认密码提示（如果没有从Pod获取到）
			if len(passwordInfo) == 0 {
				accessInfo["default_password"] = "8Dd8dw8k"
				accessInfo["password_note"] = "无法从Pod获取密码信息，使用默认密码"
			}
		}
	}
	envStatus["access"] = accessInfo

	// 分析错误情况
	allErrors := append(criticalErrors, permissionErrors...)

	if len(allErrors) > 0 {
		// 构建错误消息
		var errorMsg string
		var httpStatus int

		if len(permissionErrors) > 0 && len(criticalErrors) == 0 {
			// 只有权限错误
			errorMsg = fmt.Sprintf("权限不足，无法访问环境信息: %s", strings.Join(allErrors, "; "))
			httpStatus = http.StatusForbidden
		} else if len(criticalErrors) > 0 && len(permissionErrors) == 0 {
			// 只有网络/认证等其他关键错误
			errorMsg = fmt.Sprintf("系统错误，无法获取环境信息: %s", strings.Join(allErrors, "; "))
			httpStatus = http.StatusInternalServerError
		} else {
			// 混合错误
			errorMsg = fmt.Sprintf("无法获取环境信息: %s", strings.Join(allErrors, "; "))
			httpStatus = http.StatusInternalServerError
		}

		c.JSON(httpStatus, APIResponse{
			Success: false,
			Message: errorMsg,
			Data:    envStatus,
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "环境信息获取成功",
		Data:    envStatus,
	})
}


func createPVCs(ctx context.Context, clientset *kubernetes.Clientset, req CreateEnvironmentRequest) error {
	pvcs := []struct {
		name string
		size string
	}{
		{req.Name + "-workspace", req.Storage.Workspace},
		{req.Name + "-vscode", req.Storage.VSCode},
	}

	for _, pvc := range pvcs {
		pvcObj := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvc.name,
				Namespace: req.Namespace,
				Labels: map[string]string{
					"app": req.Name,
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(pvc.size),
					},
				},
				StorageClassName: func() *string { s := "local"; return &s }(),
			},
		}

		// 尝试创建，如果已存在则检查是否需要更新
		_, err := clientset.CoreV1().PersistentVolumeClaims(req.Namespace).Create(ctx, pvcObj, metav1.CreateOptions{})
		if err != nil {
			if isAlreadyExistsError(err) {
				// 获取现有PVC
				existing, err := clientset.CoreV1().PersistentVolumeClaims(req.Namespace).Get(ctx, pvc.name, metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("获取现有PVC失败: %v", err)
				}

				// 检查存储大小是否需要更新（只允许增加存储）
				existingStorage := existing.Spec.Resources.Requests.Storage()
				newStorage := pvcObj.Spec.Resources.Requests.Storage()

				if existingStorage.Cmp(*newStorage) < 0 {
					// 新的存储更大，需要更新
					fmt.Printf("⚠️  PVC %s 存储从 %s 扩展到 %s\n", pvc.name, existingStorage.String(), newStorage.String())
					// 注意：K8s不支持缩小PVC存储，只能扩展
					// 这里我们保持原有逻辑，不更新PVC，因为存储扩展比较复杂
					fmt.Printf("ℹ️  PVC %s 保持现有存储大小 %s\n", pvc.name, existingStorage.String())
				} else {
					fmt.Printf("✅ PVC %s 已存在，存储大小合适\n", pvc.name)
				}
			} else {
				return fmt.Errorf("创建PVC失败: %v", err)
			}
		} else {
			fmt.Printf("✅ PVC %s 已创建\n", pvc.name)
		}
	}
	return nil
}

func createDeployment(ctx context.Context, clientset *kubernetes.Clientset, req CreateEnvironmentRequest) error {
	deployment := generateDeploymentYAML(req)
	deploymentObj := &appsv1.Deployment{}

	err := sigsyaml.Unmarshal([]byte(deployment), deploymentObj)
	if err != nil {
		return fmt.Errorf("解析deployment失败: %v", err)
	}

	// 尝试创建，如果已存在则更新
	_, err = clientset.AppsV1().Deployments(req.Namespace).Create(ctx, deploymentObj, metav1.CreateOptions{})
	if err != nil {
		if isAlreadyExistsError(err) {
			// 获取现有资源
			existing, err := clientset.AppsV1().Deployments(req.Namespace).Get(ctx, deploymentObj.Name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("获取现有deployment失败: %v", err)
			}

			// 保留资源版本
			deploymentObj.ResourceVersion = existing.ResourceVersion

			// 更新资源
			_, err = clientset.AppsV1().Deployments(req.Namespace).Update(ctx, deploymentObj, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("更新deployment失败: %v", err)
			}
			fmt.Printf("✅ Deployment %s 已更新\n", deploymentObj.Name)
		} else {
			return fmt.Errorf("创建deployment失败: %v", err)
		}
	} else {
		fmt.Printf("✅ Deployment %s 已创建\n", deploymentObj.Name)
	}
	return nil
}

func createService(ctx context.Context, clientset *kubernetes.Clientset, req CreateEnvironmentRequest) error {
	service := generateServiceYAML(req)
	serviceObj := &corev1.Service{}

	err := sigsyaml.Unmarshal([]byte(service), serviceObj)
	if err != nil {
		return fmt.Errorf("解析service失败: %v", err)
	}

	// 尝试创建，如果已存在则更新
	_, err = clientset.CoreV1().Services(req.Namespace).Create(ctx, serviceObj, metav1.CreateOptions{})
	if err != nil {
		if isAlreadyExistsError(err) {
			// 获取现有资源
			existing, err := clientset.CoreV1().Services(req.Namespace).Get(ctx, serviceObj.Name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("获取现有service失败: %v", err)
			}

			// 保留资源版本和cluster IP（这些是系统分配的，不能修改）
			serviceObj.ResourceVersion = existing.ResourceVersion
			serviceObj.Spec.ClusterIP = existing.Spec.ClusterIP

			// 更新资源
			_, err = clientset.CoreV1().Services(req.Namespace).Update(ctx, serviceObj, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("更新service失败: %v", err)
			}
			fmt.Printf("✅ Service %s 已更新\n", serviceObj.Name)
		} else {
			return fmt.Errorf("创建service失败: %v", err)
		}
	} else {
		fmt.Printf("✅ Service %s 已创建\n", serviceObj.Name)
	}
	return nil
}