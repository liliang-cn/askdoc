package service

import (
	"context"

	"github.com/liliang-cn/askdoc/internal/config"
	"github.com/liliang-cn/askdoc/internal/domain"
	"github.com/liliang-cn/askdoc/internal/repository"
)

// WidgetConfigResponse is the response for widget config
type WidgetConfigResponse struct {
	SiteID  string              `json:"site_id"`
	Name    string              `json:"name"`
	Config  domain.WidgetConfig `json:"config"`
	BaseURL string              `json:"base_url"`
}

// WidgetService handles widget operations
type WidgetService struct {
	cfg         *config.Config
	siteRepo    *repository.SiteRepository
	sessionRepo *repository.SessionRepository
	chatService *ChatService
}

// NewWidgetService creates a new widget service
func NewWidgetService(
	cfg *config.Config,
	siteRepo *repository.SiteRepository,
	sessionRepo *repository.SessionRepository,
	chatService *ChatService,
) *WidgetService {
	return &WidgetService{
		cfg:         cfg,
		siteRepo:    siteRepo,
		sessionRepo: sessionRepo,
		chatService: chatService,
	}
}

// GetWidgetConfig returns the widget configuration for a site
// requestHost is the Host header from the incoming request, used to generate a dynamic base_url
// so that LAN clients get the correct URL instead of localhost.
func (s *WidgetService) GetWidgetConfig(ctx context.Context, siteID string, requestScheme, requestHost string) (*WidgetConfigResponse, error) {
	site, err := s.siteRepo.Get(siteID)
	if err != nil {
		return nil, err
	}
	if site == nil {
		return nil, domain.ErrNotFound
	}

	// Derive base_url from the request so LAN clients get the right address
	baseURL := s.cfg.Server.BaseURL
	if requestHost != "" {
		scheme := requestScheme
		if scheme == "" {
			scheme = "http"
		}
		baseURL = scheme + "://" + requestHost
	}

	return &WidgetConfigResponse{
		SiteID:  site.ID,
		Name:    site.Name,
		Config:  site.WidgetConfig,
		BaseURL: baseURL,
	}, nil
}

// Chat handles a chat message
func (s *WidgetService) Chat(ctx context.Context, siteID string, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	return s.chatService.Chat(ctx, siteID, req)
}

// ChatStream handles a streaming chat message
func (s *WidgetService) ChatStream(ctx context.Context, siteID string, req *domain.ChatRequest) (<-chan domain.StreamChunk, error) {
	return s.chatService.ChatStream(ctx, siteID, req)
}
