package api

import (
	"embed"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// StaticFS holds the embedded static files
var StaticFS embed.FS

// SetupStaticRoutes sets up routes for serving static files
func SetupStaticRoutes(r *gin.Engine) error {
	// Serve SDK
	r.GET("/sdk.js", func(c *gin.Context) {
		c.Header("Content-Type", "application/javascript")
		c.FileFromFS("static/sdk.js", http.FS(StaticFS))
	})

	// Serve admin UI - use single catch-all route that handles all /admin/* paths
	r.GET("/admin/*filepath", func(c *gin.Context) {
		path := c.Param("filepath")
		// Remove leading slash
		if len(path) > 0 && path[0] == '/' {
			path = path[1:]
		}
		// Default to index.html for root or empty path
		if path == "" || path == "/" {
			path = "index.html"
		}
		serveAdminFile(c, path)
	})

	return nil
}

func serveAdminFile(c *gin.Context, filename string) {
	fullPath := "static/admin/" + filename

	file, err := StaticFS.Open(fullPath)
	if err != nil {
		c.String(http.StatusNotFound, "File not found")
		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to read file")
		return
	}

	contentType := "text/html; charset=utf-8"
	if len(filename) > 3 && filename[len(filename)-3:] == ".js" {
		contentType = "application/javascript"
	} else if len(filename) > 4 && filename[len(filename)-4:] == ".css" {
		contentType = "text/css"
	}

	c.Data(http.StatusOK, contentType, content)
}
