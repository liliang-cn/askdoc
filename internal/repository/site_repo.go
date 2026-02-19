package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/liliang-cn/askdoc/internal/domain"
)

// SiteRepository handles site persistence
type SiteRepository struct {
	db *DB
}

// NewSiteRepository creates a new site repository
func NewSiteRepository(db *DB) *SiteRepository {
	return &SiteRepository{db: db}
}

// Create creates a new site
func (r *SiteRepository) Create(site *domain.Site) error {
	if site.ID == "" {
		site.ID = uuid.New().String()
	}
	now := time.Now()
	site.CreatedAt = now
	site.UpdatedAt = now

	collectionIDsJSON, _ := json.Marshal(site.CollectionIDs)
	widgetConfigJSON, _ := json.Marshal(site.WidgetConfig)

	_, err := r.db.Exec(`
		INSERT INTO sites (id, name, domain, collection_ids, widget_config, rate_limit, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, site.ID, site.Name, site.Domain, string(collectionIDsJSON),
		string(widgetConfigJSON), site.RateLimit, site.CreatedAt, site.UpdatedAt)

	return err
}

// Get retrieves a site by ID
func (r *SiteRepository) Get(id string) (*domain.Site, error) {
	site := &domain.Site{}
	var collectionIDsJSON, widgetConfigJSON string

	err := r.db.QueryRow(`
		SELECT id, name, domain, collection_ids, widget_config, rate_limit, created_at, updated_at
		FROM sites WHERE id = ?
	`, id).Scan(&site.ID, &site.Name, &site.Domain, &collectionIDsJSON,
		&widgetConfigJSON, &site.RateLimit, &site.CreatedAt, &site.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(collectionIDsJSON), &site.CollectionIDs)
	json.Unmarshal([]byte(widgetConfigJSON), &site.WidgetConfig)

	return site, nil
}

// List retrieves all sites
func (r *SiteRepository) List() ([]*domain.Site, error) {
	rows, err := r.db.Query(`
		SELECT id, name, domain, collection_ids, widget_config, rate_limit, created_at, updated_at
		FROM sites ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sites []*domain.Site
	for rows.Next() {
		site := &domain.Site{}
		var collectionIDsJSON, widgetConfigJSON string

		if err := rows.Scan(&site.ID, &site.Name, &site.Domain, &collectionIDsJSON,
			&widgetConfigJSON, &site.RateLimit, &site.CreatedAt, &site.UpdatedAt); err != nil {
			return nil, err
		}

		json.Unmarshal([]byte(collectionIDsJSON), &site.CollectionIDs)
		json.Unmarshal([]byte(widgetConfigJSON), &site.WidgetConfig)
		sites = append(sites, site)
	}

	return sites, rows.Err()
}

// Update updates a site
func (r *SiteRepository) Update(site *domain.Site) error {
	site.UpdatedAt = time.Now()
	collectionIDsJSON, _ := json.Marshal(site.CollectionIDs)
	widgetConfigJSON, _ := json.Marshal(site.WidgetConfig)

	result, err := r.db.Exec(`
		UPDATE sites SET name = ?, domain = ?, collection_ids = ?, widget_config = ?, rate_limit = ?, updated_at = ?
		WHERE id = ?
	`, site.Name, site.Domain, string(collectionIDsJSON),
		string(widgetConfigJSON), site.RateLimit, site.UpdatedAt, site.ID)

	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("site not found: %s", site.ID)
	}

	return nil
}

// Delete deletes a site
func (r *SiteRepository) Delete(id string) error {
	result, err := r.db.Exec(`DELETE FROM sites WHERE id = ?`, id)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("site not found: %s", id)
	}

	return nil
}
