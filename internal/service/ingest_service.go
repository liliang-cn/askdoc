package service

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/liliang-cn/askdoc/internal/config"
	"github.com/liliang-cn/askdoc/internal/domain"
	"github.com/liliang-cn/askdoc/internal/repository"
)

// IngestService handles document ingestion using rago storage
type IngestService struct {
	collectionRepo *repository.CollectionRepository
	cfg            *config.Config
	orchestrator   *OrchestratorService
}

// NewIngestService creates a new ingest service
func NewIngestService(
	collectionRepo *repository.CollectionRepository,
	cfg *config.Config,
	orchestrator *OrchestratorService,
) *IngestService {
	return &IngestService{
		collectionRepo: collectionRepo,
		cfg:            cfg,
		orchestrator:   orchestrator,
	}
}

// FileType constants
const (
	FileTypePDF  = "pdf"
	FileTypeMD   = "md"
	FileTypeTXT  = "txt"
	FileTypeHTML = "html"
	FileTypeADOC = "adoc"
)

// DetectFileType detects file type from filename
func DetectFileType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".pdf":
		return FileTypePDF
	case ".md", ".markdown":
		return FileTypeMD
	case ".txt":
		return FileTypeTXT
	case ".html", ".htm":
		return FileTypeHTML
	case ".adoc", ".asciidoc":
		return FileTypeADOC
	default:
		return ext[1:] // remove leading dot
	}
}

// IsSupported checks if file type is supported
func IsSupported(fileType string) bool {
	supported := map[string]bool{
		FileTypePDF:  true,
		FileTypeMD:   true,
		FileTypeTXT:  true,
		FileTypeHTML: true,
		FileTypeADOC: true,
	}
	return supported[fileType]
}

// UploadDocument uploads and queues a document for ingestion
func (s *IngestService) UploadDocument(
	ctx context.Context,
	collectionID string,
	file *multipart.FileHeader,
	metadata map[string]any,
) (*domain.Document, error) {
	// Check collection exists
	collection, err := s.collectionRepo.Get(collectionID)
	if err != nil {
		return nil, err
	}
	if collection == nil {
		return nil, fmt.Errorf("collection not found: %s", collectionID)
	}

	// Detect file type
	fileType := DetectFileType(file.Filename)
	if !IsSupported(fileType) {
		return nil, fmt.Errorf("unsupported file type: %s", fileType)
	}

	// Create storage directory
	storageDir := filepath.Join(s.cfg.Storage.Documents, collectionID)
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Generate unique document ID
	docID := uuid.New().String()
	ext := filepath.Ext(file.Filename)
	storagePath := filepath.Join(storageDir, docID+ext)

	// Save file
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(storagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	// Update collection document count
	if err := s.collectionRepo.UpdateDocumentCount(collectionID, 1); err != nil {
		return nil, err
	}

	// Create document record (will be stored in rago after ingestion)
	document := &domain.Document{
		ID:           docID,
		CollectionID: collectionID,
		Filename:     file.Filename,
		FileType:     fileType,
		FileSize:     file.Size,
		Status:       domain.DocumentStatusPending,
		Metadata:     metadata,
	}

	// Start async ingestion using Orchestrator
	go s.ingestDocument(context.Background(), document, storagePath)

	return document, nil
}

// ingestDocument processes a document and ingests it into rago storage
func (s *IngestService) ingestDocument(ctx context.Context, document *domain.Document, storagePath string) {
	// Build metadata for rago - includes all AskDoc-specific fields
	metadata := make(map[string]any)
	metadata[domain.MetadataKeyCollectionID] = document.CollectionID
	metadata[domain.MetadataKeyFilename] = document.Filename
	metadata[domain.MetadataKeyFileType] = document.FileType
	metadata[domain.MetadataKeyFileSize] = document.FileSize
	metadata[domain.MetadataKeyStatus] = domain.DocumentStatusProcessing
	for k, v := range document.Metadata {
		metadata[k] = v
	}

	var chunkCount int
	var ingestErr error

	if s.orchestrator != nil {
		// Ingest using Orchestrator (stores document in rago)
		resp, err := s.orchestrator.IngestFile(ctx, storagePath, metadata)
		if err != nil {
			ingestErr = err
		} else {
			chunkCount = resp.ChunkCount
			// Update document ID to match rago's document ID
			document.ID = resp.DocumentID

			// Update metadata with chunk count and status
			updateMeta := map[string]any{
				domain.MetadataKeyChunkCount: chunkCount,
				domain.MetadataKeyStatus:     domain.DocumentStatusReady,
			}
			s.orchestrator.UpdateDocumentMetadata(ctx, document.ID, updateMeta)
		}
	} else {
		// No orchestrator service, just mark as ready with 0 chunks
		chunkCount = 0
	}

	// Handle ingestion error
	if ingestErr != nil {
		// Update metadata with error status
		if s.orchestrator != nil {
			updateMeta := map[string]any{
				domain.MetadataKeyStatus: domain.DocumentStatusFailed,
				domain.MetadataKeyError:  ingestErr.Error(),
			}
			s.orchestrator.UpdateDocumentMetadata(ctx, document.ID, updateMeta)
		}
		document.Status = domain.DocumentStatusFailed
		document.Error = ingestErr.Error()
	} else {
		document.Status = domain.DocumentStatusReady
		document.ChunkCount = chunkCount
	}
}

// GetStoragePath returns the storage path for a document
func (s *IngestService) GetStoragePath(doc *domain.Document) string {
	ext := filepath.Ext(doc.Filename)
	return filepath.Join(s.cfg.Storage.Documents, doc.CollectionID, doc.ID+ext)
}

// GetDocument retrieves a document from rago storage
func (s *IngestService) GetDocument(ctx context.Context, id string) (*domain.Document, error) {
	if s.orchestrator == nil {
		return nil, fmt.Errorf("orchestrator not available")
	}
	return s.orchestrator.GetDocument(ctx, id)
}

// ListDocumentsByCollection lists documents for a collection from rago storage
func (s *IngestService) ListDocumentsByCollection(ctx context.Context, collectionID string, page, pageSize int) ([]*domain.Document, int, error) {
	if s.orchestrator == nil {
		return nil, 0, fmt.Errorf("orchestrator not available")
	}

	docs, err := s.orchestrator.ListDocumentsByCollection(ctx, collectionID)
	if err != nil {
		return nil, 0, err
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

	if start >= total {
		return []*domain.Document{}, total, nil
	}

	return docs[start:end], total, nil
}

// DeleteDocument deletes a document from rago storage and file system
func (s *IngestService) DeleteDocument(ctx context.Context, id string, collectionID string) error {
	if s.orchestrator == nil {
		return fmt.Errorf("orchestrator not available")
	}

	// Delete from rago storage
	if err := s.orchestrator.DeleteDocument(ctx, id); err != nil {
		return err
	}

	// Delete from file system
	storagePath := filepath.Join(s.cfg.Storage.Documents, collectionID, id)
	// Try common extensions
	for _, ext := range []string{".txt", ".pdf", ".md", ".html"} {
		if err := os.Remove(storagePath + ext); err == nil {
			break
		}
	}

	// Update collection document count
	return s.collectionRepo.UpdateDocumentCount(collectionID, -1)
}
