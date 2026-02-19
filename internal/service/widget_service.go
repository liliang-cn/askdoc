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
	cfg          *config.Config
	siteRepo     *repository.SiteRepository
	sessionRepo  *repository.SessionRepository
	chatService  *ChatService
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
func (s *WidgetService) GetWidgetConfig(ctx context.Context, siteID string) (*WidgetConfigResponse, error) {
	site, err := s.siteRepo.Get(siteID)
	if err != nil {
		return nil, err
	}
	if site == nil {
		return nil, domain.ErrNotFound
	}

	return &WidgetConfigResponse{
		SiteID:  site.ID,
		Name:    site.Name,
		Config:  site.WidgetConfig,
		BaseURL: s.cfg.Server.BaseURL,
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
