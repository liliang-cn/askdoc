package widget

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/liliang-cn/askdoc/internal/domain"
	"github.com/liliang-cn/askdoc/internal/service"
)

// Handler handles widget API requests
type Handler struct {
	widgetService *service.WidgetService
}

// NewHandler creates a new widget handler
func NewHandler(widgetService *service.WidgetService) *Handler {
	return &Handler{widgetService: widgetService}
}

// RegisterRoutes registers widget routes
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/config/:site_id", h.GetConfig)
	r.POST("/chat/:site_id", h.Chat)
	r.POST("/chat/:site_id/stream", h.ChatStream)
}

// GetConfig returns the widget configuration for a site
func (h *Handler) GetConfig(c *gin.Context) {
	siteID := c.Param("site_id")

	// Determine scheme from request (support reverse proxy headers)
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}

	config, err := h.widgetService.GetWidgetConfig(c.Request.Context(), siteID, scheme, c.Request.Host)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "site not found"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// Chat handles a chat message
func (h *Handler) Chat(c *gin.Context) {
	siteID := c.Param("site_id")

	var req domain.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.widgetService.Chat(c.Request.Context(), siteID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ChatStream handles a streaming chat message (SSE)
func (h *Handler) ChatStream(c *gin.Context) {
	siteID := c.Param("site_id")

	var req domain.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	stream, err := h.widgetService.ChatStream(c.Request.Context(), siteID, &req)
	if err != nil {
		writeSSE(c.Writer, "error", err.Error())
		return
	}

	// Use gin.Stream for SSE
	c.Stream(func(w io.Writer) bool {
		select {
		case chunk, ok := <-stream:
			if !ok {
				return false // End stream
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", chunk.Type, string(data))
			return true
		default:
			return true // Keep stream open
		}
	})
}

func writeSSE(w io.Writer, eventType, data string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, data)
}
