package domain

import "time"

// Collection represents a document collection
type Collection struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	DocumentCount int            `json:"document_count"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// CreateCollectionRequest is the request to create a collection
type CreateCollectionRequest struct {
	Name        string         `json:"name" binding:"required"`
	Description string         `json:"description,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// UpdateCollectionRequest is the request to update a collection
type UpdateCollectionRequest struct {
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}
