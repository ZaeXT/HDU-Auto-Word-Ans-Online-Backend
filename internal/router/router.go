package router

import (
	"HDU-Auto-Word-Ans-Online-Backend/internal/api"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func SetupRouter(examHandler *api.ExamHandler, allowedOrigins []string) *gin.Engine {
	r := gin.Default()

	config := cors.DefaultConfig()
	config.AllowOrigins = allowedOrigins
	config.AllowHeaders = append(config.AllowHeaders, "X-Auth-Token", "Content-Type")
	r.Use(cors.New(config))

	apiV1 := r.Group("/api/v1")
	{
		apiV1.POST("/start-test", examHandler.StartTestHandler)
		apiV1.POST("/login-and-start", examHandler.LoginAndStartTestHandler)
		apiV1.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "UP"})
		})
	}

	return r
}
