package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/liliang-cn/askdoc/internal/domain"
	"github.com/liliang-cn/askdoc/internal/service"
)

// Handler handles admin API requests
type Handler struct {
	adminService  *service.AdminService
	ingestService *service.IngestService
}

// NewHandler creates a new admin handler
func NewHandler(adminService *service.AdminService, ingestService *service.IngestService) *Handler {
	return &Handler{
		adminService:  adminService,
		ingestService: ingestService,
	}
}

// RegisterRoutes registers admin routes
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	collections := r.Group("/collections")
	{
		collections.POST("", h.CreateCollection)
		collections.GET("", h.ListCollections)
		collections.GET("/:id", h.GetCollection)
		collections.PUT("/:id", h.UpdateCollection)
		collections.DELETE("/:id", h.DeleteCollection)
		collections.POST("/:id/documents", h.UploadDocument)
		collections.GET("/:id/documents", h.ListDocuments)
	}

	documents := r.Group("/documents")
	{
		documents.GET("/:id", h.GetDocument)
		documents.DELETE("/:id", h.DeleteDocument)
	}

	sites := r.Group("/sites")
	{
		sites.POST("", h.CreateSite)
		sites.GET("", h.ListSites)
		sites.GET("/:id", h.GetSite)
		sites.PUT("/:id", h.UpdateSite)
		sites.DELETE("/:id", h.DeleteSite)
	}

	r.GET("/stats", h.GetStats)
}

// Collection handlers

func (h *Handler) CreateCollection(c *gin.Context) {
	var req domain.CreateCollectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	collection, err := h.adminService.CreateCollection(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, collection)
}

func (h *Handler) ListCollections(c *gin.Context) {
	collections, err := h.adminService.ListCollections(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"collections": collections})
}

func (h *Handler) GetCollection(c *gin.Context) {
	id := c.Param("id")
	collection, err := h.adminService.GetCollection(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if collection == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "collection not found"})
		return
	}

	c.JSON(http.StatusOK, collection)
}

func (h *Handler) UpdateCollection(c *gin.Context) {
	id := c.Param("id")
	var req domain.UpdateCollectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	collection, err := h.adminService.UpdateCollection(c.Request.Context(), id, &req)
	if err != nil {
		if err == domain.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "collection not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, collection)
}

func (h *Handler) DeleteCollection(c *gin.Context) {
	id := c.Param("id")
	if err := h.adminService.DeleteCollection(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "collection deleted"})
}

// Document handlers

func (h *Handler) UploadDocument(c *gin.Context) {
	collectionID := c.Param("id")

	// Get file from form
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	// Parse metadata if provided
	metadata := make(map[string]any)
	if metaStr := c.PostForm("metadata"); metaStr != "" {
		if err := json.Unmarshal([]byte(metaStr), &metadata); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid metadata JSON"})
			return
		}
	}

	// Upload document
	document, err := h.ingestService.UploadDocument(c.Request.Context(), collectionID, file, metadata)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, document)
}

func (h *Handler) ListDocuments(c *gin.Context) {
	collectionID := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	result, err := h.adminService.ListDocuments(c.Request.Context(), collectionID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetDocument(c *gin.Context) {
	id := c.Param("id")
	document, err := h.adminService.GetDocument(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if document == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}

	c.JSON(http.StatusOK, document)
}

func (h *Handler) DeleteDocument(c *gin.Context) {
	id := c.Param("id")
	if err := h.adminService.DeleteDocument(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "document deleted"})
}

// Site handlers

func (h *Handler) CreateSite(c *gin.Context) {
	var req domain.CreateSiteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	site, err := h.adminService.CreateSite(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, site)
}

func (h *Handler) ListSites(c *gin.Context) {
	sites, err := h.adminService.ListSites(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"sites": sites})
}

func (h *Handler) GetSite(c *gin.Context) {
	id := c.Param("id")
	site, err := h.adminService.GetSite(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if site == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "site not found"})
		return
	}

	c.JSON(http.StatusOK, site)
}

func (h *Handler) UpdateSite(c *gin.Context) {
	id := c.Param("id")
	var req domain.UpdateSiteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	site, err := h.adminService.UpdateSite(c.Request.Context(), id, &req)
	if err != nil {
		if err == domain.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "site not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, site)
}

func (h *Handler) DeleteSite(c *gin.Context) {
	id := c.Param("id")
	if err := h.adminService.DeleteSite(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "site deleted"})
}

// Stats handler

func (h *Handler) GetStats(c *gin.Context) {
	stats, err := h.adminService.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}
