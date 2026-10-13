package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"menu-tree-backend/internal/handler"
)

func New(menuHandler *handler.MenuHandler) *gin.Engine {
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	menus := r.Group("/api/menus")
	{
		menus.GET("", menuHandler.GetAll)
		menus.GET("/:id", menuHandler.GetByID)
		menus.POST("", menuHandler.Create)
		menus.PUT("/:id", menuHandler.Update)
		menus.DELETE("/:id", menuHandler.Delete)
	}

	return r
}
