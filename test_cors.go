package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	// 创建Gin路由
	r := gin.Default()

	// 添加详细的CORS中间件
	r.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		method := c.Request.Method
		path := c.Request.URL.Path

		log.Printf("=== CORS Debug ===")
		log.Printf("Origin: %s", origin)
		log.Printf("Method: %s", method)
		log.Printf("Path: %s", path)
		log.Printf("Headers: %v", c.Request.Header)

		// 设置CORS头
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Requested-With")
		c.Header("Access-Control-Max-Age", "86400")

		// 处理预检请求
		if method == "OPTIONS" {
			log.Printf("Handling OPTIONS request")
			c.AbortWithStatus(204)
			return
		}

		log.Printf("CORS headers set successfully")
		log.Printf("==================")

		c.Next()
	})

	// 测试路由
	r.GET("/test-cors", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "CORS test successful",
			"time":    time.Now(),
			"headers": c.Request.Header,
		})
	})

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now(),
		})
	})

	log.Println("CORS test server starting on :8080")
	log.Fatal(r.Run(":8080"))
}