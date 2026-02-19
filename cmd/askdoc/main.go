package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/liliang-cn/askdoc/internal/api"
	"github.com/liliang-cn/askdoc/internal/config"
	"github.com/liliang-cn/askdoc/internal/repository"
	"github.com/liliang-cn/askdoc/internal/service"
	"go.uber.org/zap"
)

var (
	configPath = flag.String("config", "", "Path to config file")
)

func main() {
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Initialize database (for collections, sites, sessions - documents are in rago)
	db, err := repository.NewDB(cfg.Database.Path)
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer db.Close()

	// Initialize repositories
	collectionRepo := repository.NewCollectionRepository(db)
	siteRepo := repository.NewSiteRepository(db)
	sessionRepo := repository.NewSessionRepository(db)

	// Initialize Orchestrator Service (integrates rago for RAG and document storage)
	orchestrator, err := service.NewOrchestratorService(cfg)
	if err != nil {
		logger.Warn("Failed to initialize Orchestrator, running without RAG", zap.Error(err))
		// Continue without orchestrator - will use placeholder responses
	}

	// Initialize services
	adminService := service.NewAdminService(
		collectionRepo,
		siteRepo,
		sessionRepo,
		orchestrator,
	)

	ingestService := service.NewIngestService(
		collectionRepo,
		cfg,
		orchestrator,
	)

	chatService := service.NewChatService(
		cfg,
		siteRepo,
		sessionRepo,
		orchestrator,
	)

	widgetService := service.NewWidgetService(
		cfg,
		siteRepo,
		sessionRepo,
		chatService,
	)

	// Setup router
	router := api.SetupRouter(adminService, ingestService, widgetService, api.RouterConfig{
		APIKey:       cfg.Admin.APIKey,
		AllowOrigins: []string{"*"},
	})

	// Create HTTP server
	srv := &http.Server{
		Addr:         cfg.Address(),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("Starting AskDoc server",
			zap.String("address", cfg.Address()),
			zap.String("base_url", cfg.Server.BaseURL),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	// Close orchestrator
	if orchestrator != nil {
		orchestrator.Close()
	}

	logger.Info("Server exited")
}

func printBanner() {
	banner := `
   ___   _____  _____
  /   | / ___/ / ___/ ____   ____
 / /| |/ __/  / __ \/ __ \/ __ \
/ ___ / /___ / /_/ // / / // /_/ /
/_/  |_/____//____/ /_/ /_/ \__, /
                           /____/
`

	fmt.Println(banner)
}
