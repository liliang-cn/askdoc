package domain

import "time"

// Site represents a JS SDK configuration
type Site struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Domain        string       `json:"domain"`
	CollectionIDs []string     `json:"collection_ids"`
	WidgetConfig  WidgetConfig `json:"widget_config"`
	RateLimit     int          `json:"rate_limit"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

// WidgetConfig holds UI configuration for the widget
type WidgetConfig struct {
	Theme          string `json:"theme"`
	PrimaryColor   string `json:"primary_color"`
	Position       string `json:"position"`
	WelcomeMessage string `json:"welcome_message"`
	Placeholder    string `json:"placeholder"`
	ShowSources    bool   `json:"show_sources"`
}

// CreateSiteRequest is the request to create a site
type CreateSiteRequest struct {
	Name          string         `json:"name" binding:"required"`
	Domain        string         `json:"domain" binding:"required"`
	CollectionIDs []string       `json:"collection_ids" binding:"required"`
	WidgetConfig  *WidgetConfig  `json:"widget_config,omitempty"`
	RateLimit     int            `json:"rate_limit,omitempty"`
}

// UpdateSiteRequest is the request to update a site
type UpdateSiteRequest struct {
	Name          string         `json:"name,omitempty"`
	Domain        string         `json:"domain,omitempty"`
	CollectionIDs []string       `json:"collection_ids,omitempty"`
	WidgetConfig  *WidgetConfig  `json:"widget_config,omitempty"`
	RateLimit     int            `json:"rate_limit,omitempty"`
}

// DefaultWidgetConfig returns default widget configuration
func DefaultWidgetConfig() WidgetConfig {
	return WidgetConfig{
		Theme:          "light",
		PrimaryColor:   "#3b82f6",
		Position:       "bottom-right",
		WelcomeMessage: "Hi! How can I help you?",
		Placeholder:    "Ask a question...",
		ShowSources:    true,
	}
}
