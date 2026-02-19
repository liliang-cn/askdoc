package api

import (
	"github.com/gin-gonic/gin"
	"github.com/liliang-cn/askdoc/internal/api/admin"
	"github.com/liliang-cn/askdoc/internal/api/middleware"
	"github.com/liliang-cn/askdoc/internal/api/widget"
	"github.com/liliang-cn/askdoc/internal/service"
)

// RouterConfig holds configuration for the router
type RouterConfig struct {
	APIKey       string
	AllowOrigins []string
}

// SetupRouter sets up the Gin router
func SetupRouter(
	adminService *service.AdminService,
	ingestService *service.IngestService,
	widgetService *service.WidgetService,
	cfg RouterConfig,
) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	// CORS middleware
	r.Use(middleware.CORS(cfg.AllowOrigins))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Static files (admin UI, SDK)
	SetupStaticRoutes(r)

	// Widget API (public, based on site_id)
	widgetHandler := widget.NewHandler(widgetService)
	widgetGroup := r.Group("/api/widget")
	widgetHandler.RegisterRoutes(widgetGroup)

	// Admin API (requires API key)
	adminHandler := admin.NewHandler(adminService, ingestService)
	adminGroup := r.Group("/api/admin")
	adminGroup.Use(middleware.Auth(cfg.APIKey))
	adminHandler.RegisterRoutes(adminGroup)

	return r
}
