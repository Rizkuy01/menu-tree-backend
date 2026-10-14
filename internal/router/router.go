package router

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"menu-tree-backend/internal/handler"
)

func New(menuHandler *handler.MenuHandler) *gin.Engine {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"http://localhost:3000"},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Content-Type"},
	}))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/swagger.json", func(c *gin.Context) {
		c.File("docs/swagger.json")
	})
	r.GET("/api/swagger.json", func(c *gin.Context) {
		c.File("docs/swagger.json")
	})

	menus := r.Group("/api/menus")
	{
		menus.GET("", menuHandler.GetAll)
		menus.GET("/:id", menuHandler.GetByID)
		menus.POST("", menuHandler.Create)
		menus.PUT("/:id", menuHandler.Update)
		menus.DELETE("/:id", menuHandler.Delete)
		menus.PATCH("/:id/move", menuHandler.Move)
		menus.PATCH("/:id/reorder", menuHandler.Reorder)
	}

	return r
}
