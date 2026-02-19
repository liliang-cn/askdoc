package service

import (
	"context"

	"github.com/liliang-cn/askdoc/internal/domain"
	"github.com/liliang-cn/askdoc/internal/repository"
)

// AdminService handles admin operations
type AdminService struct {
	collectionRepo *repository.CollectionRepository
	siteRepo       *repository.SiteRepository
	sessionRepo    *repository.SessionRepository
	orchestrator   *OrchestratorService
}

// NewAdminService creates a new admin service
func NewAdminService(
	collectionRepo *repository.CollectionRepository,
	siteRepo *repository.SiteRepository,
	sessionRepo *repository.SessionRepository,
	orchestrator *OrchestratorService,
) *AdminService {
	return &AdminService{
		collectionRepo: collectionRepo,
		siteRepo:       siteRepo,
		sessionRepo:    sessionRepo,
		orchestrator:   orchestrator,
	}
}

// Collection operations

func (s *AdminService) CreateCollection(ctx context.Context, req *domain.CreateCollectionRequest) (*domain.Collection, error) {
	collection := &domain.Collection{
		Name:        req.Name,
		Description: req.Description,
		Metadata:    req.Metadata,
	}
	if err := s.collectionRepo.Create(collection); err != nil {
		return nil, err
	}
	return collection, nil
}

func (s *AdminService) GetCollection(ctx context.Context, id string) (*domain.Collection, error) {
	return s.collectionRepo.Get(id)
}

func (s *AdminService) ListCollections(ctx context.Context) ([]*domain.Collection, error) {
	return s.collectionRepo.List()
}

func (s *AdminService) UpdateCollection(ctx context.Context, id string, req *domain.UpdateCollectionRequest) (*domain.Collection, error) {
	collection, err := s.collectionRepo.Get(id)
	if err != nil {
		return nil, err
	}
	if collection == nil {
		return nil, domain.ErrNotFound
	}

	if req.Name != "" {
		collection.Name = req.Name
	}
	if req.Description != "" {
		collection.Description = req.Description
	}
	if req.Metadata != nil {
		collection.Metadata = req.Metadata
	}

	if err := s.collectionRepo.Update(collection); err != nil {
		return nil, err
	}
	return collection, nil
}

func (s *AdminService) DeleteCollection(ctx context.Context, id string) error {
	return s.collectionRepo.Delete(id)
}

// Document operations (delegated to IngestService via orchestrator)

func (s *AdminService) GetDocument(ctx context.Context, id string) (*domain.Document, error) {
	if s.orchestrator == nil {
		return nil, domain.ErrNotFound
	}
	return s.orchestrator.GetDocument(ctx, id)
}

func (s *AdminService) ListDocuments(ctx context.Context, collectionID string, page, pageSize int) (*domain.DocumentListResponse, error) {
	if s.orchestrator == nil {
		return &domain.DocumentListResponse{Documents: []*domain.Document{}, Total: 0, Page: page, PageSize: pageSize}, nil
	}

	docs, err := s.orchestrator.ListDocumentsByCollection(ctx, collectionID)
	if err != nil {
		return nil, err
	}

	// Pagination
	total := len(docs)
	start := (page - 1) * pageSize
	if start < 0 {
		start = 0
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	var pagedDocs []*domain.Document
	if start < total {
		pagedDocs = docs[start:end]
	} else {
		pagedDocs = []*domain.Document{}
	}

	return &domain.DocumentListResponse{
		Documents: pagedDocs,
		Total:     total,
		Page:      page,
		PageSize:  pageSize,
	}, nil
}

func (s *AdminService) DeleteDocument(ctx context.Context, id string) error {
	if s.orchestrator == nil {
		return domain.ErrNotFound
	}
	return s.orchestrator.DeleteDocument(ctx, id)
}

// Site operations

func (s *AdminService) CreateSite(ctx context.Context, req *domain.CreateSiteRequest) (*domain.Site, error) {
	site := &domain.Site{
		Name:          req.Name,
		Domain:        req.Domain,
		CollectionIDs: req.CollectionIDs,
		RateLimit:     req.RateLimit,
	}

	if req.WidgetConfig != nil {
		site.WidgetConfig = *req.WidgetConfig
	} else {
		site.WidgetConfig = domain.DefaultWidgetConfig()
	}

	if site.RateLimit == 0 {
		site.RateLimit = 100
	}

	if err := s.siteRepo.Create(site); err != nil {
		return nil, err
	}
	return site, nil
}

func (s *AdminService) GetSite(ctx context.Context, id string) (*domain.Site, error) {
	return s.siteRepo.Get(id)
}

func (s *AdminService) ListSites(ctx context.Context) ([]*domain.Site, error) {
	return s.siteRepo.List()
}

func (s *AdminService) UpdateSite(ctx context.Context, id string, req *domain.UpdateSiteRequest) (*domain.Site, error) {
	site, err := s.siteRepo.Get(id)
	if err != nil {
		return nil, err
	}
	if site == nil {
		return nil, domain.ErrNotFound
	}

	if req.Name != "" {
		site.Name = req.Name
	}
	if req.Domain != "" {
		site.Domain = req.Domain
	}
	if req.CollectionIDs != nil {
		site.CollectionIDs = req.CollectionIDs
	}
	if req.WidgetConfig != nil {
		site.WidgetConfig = *req.WidgetConfig
	}
	if req.RateLimit > 0 {
		site.RateLimit = req.RateLimit
	}

	if err := s.siteRepo.Update(site); err != nil {
		return nil, err
	}
	return site, nil
}

func (s *AdminService) DeleteSite(ctx context.Context, id string) error {
	return s.siteRepo.Delete(id)
}

// Stats

func (s *AdminService) GetStats(ctx context.Context) (*domain.Stats, error) {
	collections, _ := s.collectionRepo.List()
	sites, _ := s.siteRepo.List()
	chats, _ := s.sessionRepo.CountChats()

	// Get document count from rago
	var docCount int
	if s.orchestrator != nil {
		docs, err := s.orchestrator.ListDocuments(ctx)
		if err == nil {
			docCount = len(docs)
		}
	}

	return &domain.Stats{
		TotalCollections: len(collections),
		TotalDocuments:   docCount,
		TotalSites:       len(sites),
		TotalChats:       chats,
	}, nil
}
