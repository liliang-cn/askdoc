package domain

import "time"

// Document status constants (stored in rago metadata)
const (
	DocumentStatusPending    = "pending"
	DocumentStatusProcessing = "processing"
	DocumentStatusReady      = "ready"
	DocumentStatusFailed     = "failed"
)

// DocumentMetadata keys stored in rago's document metadata
const (
	MetadataKeyCollectionID = "collection_id"
	MetadataKeyFilename     = "filename"
	MetadataKeyFileType     = "file_type"
	MetadataKeyFileSize     = "file_size"
	MetadataKeyStatus       = "status"
	MetadataKeyChunkCount   = "chunk_count"
	MetadataKeyError        = "error"
)

// Document represents a document (API response type, backed by rago storage)
type Document struct {
	ID           string         `json:"id"`
	CollectionID string         `json:"collection_id"`
	Filename     string         `json:"filename"`
	FileType     string         `json:"file_type"`
	FileSize     int64          `json:"file_size"`
	Status       string         `json:"status"`
	ChunkCount   int            `json:"chunk_count"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	Error        string         `json:"error,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at,omitempty"`
}

// CreateDocumentRequest is the request to upload a document
type CreateDocumentRequest struct {
	CollectionID string         `form:"collection_id" binding:"required"`
	Metadata     map[string]any `form:"metadata"`
}

// DocumentListResponse is the response for listing documents
type DocumentListResponse struct {
	Documents []*Document `json:"documents"`
	Total     int         `json:"total"`
	Page      int         `json:"page"`
	PageSize  int         `json:"page_size"`
}
