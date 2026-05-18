package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	docs "github.com/anuragrao/aidocs/api/internal/server/docs"
)

func registerAPIDocsRoutes(r *gin.Engine) {
	docs.SwaggerInfo.BasePath = "/"
	r.GET("/api-docs", func(c *gin.Context) { c.Redirect(http.StatusFound, "/api-docs/index.html") })
	r.GET("/api-docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/openapi.json", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(docs.SwaggerInfo.ReadDoc()))
	})
}
