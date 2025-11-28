package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	_ "k8s-resource-api/docs/swagger"
)

// @title K8S Resource API
// @version 1.0
// @description Kubernetes资源管理API，支持动态创建开发环境和SA权限管理
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api/v1/k8s

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

func main() {
	// 创建Gin路由
	r := gin.Default()

	// 添加CORS中间件
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// API v1 路由组
	v1 := r.Group("/api/v1/k8s")
	{
		// ServiceAccount管理
		v1.POST("/service-accounts/:name", CreateServiceAccount)
		v1.DELETE("/service-accounts/:name", DeleteServiceAccount)

	// 环境管理
		v1.POST("/environments", CreateEnvironment)
		v1.POST("/environments/:name", GetEnvironment)
		v1.DELETE("/environments/:name", DeleteEnvironment)
	}

	// Swagger文档
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "k8s-resource-api",
		})
	})

	port := "8080"
	log.Printf("服务器启动在端口 %s", port)
	log.Printf("Swagger文档: http://localhost:%s/swagger/index.html", port)
	log.Fatal(r.Run(":" + port))
}