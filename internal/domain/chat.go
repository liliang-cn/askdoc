package domain

import "time"

// Session represents a chat session
type Session struct {
	ID        string    `json:"id"`
	SiteID    string    `json:"site_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Message represents a chat message
type Message struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"` // user, assistant
	Content   string    `json:"content"`
	Sources   []Source  `json:"sources,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Source represents a citation source
type Source struct {
	DocumentID string  `json:"document_id"`
	Filename   string  `json:"filename"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
}

// ChatRequest is the request to send a chat message
type ChatRequest struct {
	SessionID string `json:"session_id,omitempty"`
	Message   string `json:"message" binding:"required"`
}

// ChatResponse is the response from a chat message
type ChatResponse struct {
	SessionID string   `json:"session_id"`
	Answer    string   `json:"answer"`
	Sources   []Source `json:"sources,omitempty"`
}

// StreamChunk represents a chunk in SSE stream
type StreamChunk struct {
	Type    string `json:"type"` // thinking, content, sources, done, error
	Content string `json:"content,omitempty"`
}

// Stats represents system statistics
type Stats struct {
	TotalDocuments  int `json:"total_documents"`
	TotalCollections int `json:"total_collections"`
	TotalSites      int `json:"total_sites"`
	TotalChats      int `json:"total_chats"`
}
