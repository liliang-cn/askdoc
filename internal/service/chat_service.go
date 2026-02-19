package service

import (
	"context"
	"fmt"

	"github.com/liliang-cn/askdoc/internal/config"
	"github.com/liliang-cn/askdoc/internal/domain"
	"github.com/liliang-cn/askdoc/internal/repository"
)

// ChatService handles chat operations using Orchestrator Agent
type ChatService struct {
	cfg           *config.Config
	siteRepo      *repository.SiteRepository
	sessionRepo   *repository.SessionRepository
	orchestrator  *OrchestratorService
}

// NewChatService creates a new chat service
func NewChatService(
	cfg *config.Config,
	siteRepo *repository.SiteRepository,
	sessionRepo *repository.SessionRepository,
	orchestrator *OrchestratorService,
) *ChatService {
	return &ChatService{
		cfg:          cfg,
		siteRepo:     siteRepo,
		sessionRepo:  sessionRepo,
		orchestrator: orchestrator,
	}
}

// Chat handles a chat message using Orchestrator Agent
func (s *ChatService) Chat(ctx context.Context, siteID string, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	// Verify site exists and get collection IDs
	site, err := s.siteRepo.Get(siteID)
	if err != nil {
		return nil, err
	}
	if site == nil {
		return nil, domain.ErrNotFound
	}

	// Get or create session
	sessionID := req.SessionID
	if sessionID == "" {
		session := &domain.Session{SiteID: siteID}
		if err := s.sessionRepo.Create(session); err != nil {
			return nil, err
		}
		sessionID = session.ID
	}

	// Save user message
	userMsg := &domain.Message{
		SessionID: sessionID,
		Role:      "user",
		Content:   req.Message,
	}
	if err := s.sessionRepo.CreateMessage(userMsg); err != nil {
		return nil, err
	}

	// Query Orchestrator Agent
	var resp *domain.ChatResponse
	if s.orchestrator != nil {
		resp, err = s.orchestrator.Chat(ctx, req.Message, site.CollectionIDs)
		if err != nil {
			// Fallback to placeholder on error
			resp = &domain.ChatResponse{
				SessionID: sessionID,
				Answer:    fmt.Sprintf("Error from Agent: %v", err),
			}
		} else {
			resp.SessionID = sessionID
		}
	} else {
		// No orchestrator service configured
		resp = &domain.ChatResponse{
			SessionID: sessionID,
			Answer:    fmt.Sprintf("Orchestrator Agent not configured. Your question: %s", req.Message),
		}
	}

	// Save assistant message
	assistantMsg := &domain.Message{
		SessionID: sessionID,
		Role:      "assistant",
		Content:   resp.Answer,
		Sources:   resp.Sources,
	}
	if err := s.sessionRepo.CreateMessage(assistantMsg); err != nil {
		return nil, err
	}

	// Update session
	if err := s.sessionRepo.Update(sessionID); err != nil {
		return nil, err
	}

	return resp, nil
}

// ChatStream handles a streaming chat message using Orchestrator Agent
func (s *ChatService) ChatStream(ctx context.Context, siteID string, req *domain.ChatRequest) (<-chan domain.StreamChunk, error) {
	// Verify site exists
	site, err := s.siteRepo.Get(siteID)
	if err != nil {
		return nil, err
	}
	if site == nil {
		return nil, domain.ErrNotFound
	}

	// Use Orchestrator Agent for streaming if available
	if s.orchestrator != nil {
		return s.orchestrator.ChatStream(ctx, req.Message, site.CollectionIDs)
	}

	// Fallback to simple streaming
	ch := make(chan domain.StreamChunk, 100)
	go func() {
		defer close(ch)
		ch <- domain.StreamChunk{Type: "thinking", Content: "Processing..."}
		ch <- domain.StreamChunk{Type: "content", Content: "Orchestrator Agent not configured."}
		ch <- domain.StreamChunk{Type: "done"}
	}()
	return ch, nil
}
