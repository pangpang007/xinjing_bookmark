package handlers

import (
	"embed"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed swagger/*
var swaggerFS embed.FS

func RegisterSwagger(g gin.IRouter) {
	g.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusFound, strings.TrimRight(c.Request.URL.Path, "/")+"/index.html")
	})
	g.GET("/swagger/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "index.html")
	})
	g.GET("/swagger/index.html", func(c *gin.Context) {
		data, err := swaggerFS.ReadFile("swagger/index.html")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})
	g.GET("/swagger/openapi.yaml", func(c *gin.Context) {
		data, err := swaggerFS.ReadFile("swagger/openapi.yaml")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, "application/yaml; charset=utf-8", data)
	})
}
