package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/liliang-cn/askdoc/internal/domain"
)

// CollectionRepository handles collection persistence
type CollectionRepository struct {
	db *DB
}

// NewCollectionRepository creates a new collection repository
func NewCollectionRepository(db *DB) *CollectionRepository {
	return &CollectionRepository{db: db}
}

// Create creates a new collection
func (r *CollectionRepository) Create(collection *domain.Collection) error {
	if collection.ID == "" {
		collection.ID = uuid.New().String()
	}
	now := time.Now()
	collection.CreatedAt = now
	collection.UpdatedAt = now

	metadataJSON, _ := json.Marshal(collection.Metadata)

	_, err := r.db.Exec(`
		INSERT INTO collections (id, name, description, metadata, document_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, collection.ID, collection.Name, collection.Description, string(metadataJSON),
		collection.DocumentCount, collection.CreatedAt, collection.UpdatedAt)

	return err
}

// Get retrieves a collection by ID
func (r *CollectionRepository) Get(id string) (*domain.Collection, error) {
	collection := &domain.Collection{}
	var metadataJSON string

	err := r.db.QueryRow(`
		SELECT id, name, description, metadata, document_count, created_at, updated_at
		FROM collections WHERE id = ?
	`, id).Scan(&collection.ID, &collection.Name, &collection.Description,
		&metadataJSON, &collection.DocumentCount, &collection.CreatedAt, &collection.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if metadataJSON != "" {
		json.Unmarshal([]byte(metadataJSON), &collection.Metadata)
	}

	return collection, nil
}

// List retrieves all collections
func (r *CollectionRepository) List() ([]*domain.Collection, error) {
	rows, err := r.db.Query(`
		SELECT id, name, description, metadata, document_count, created_at, updated_at
		FROM collections ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var collections []*domain.Collection
	for rows.Next() {
		collection := &domain.Collection{}
		var metadataJSON string

		if err := rows.Scan(&collection.ID, &collection.Name, &collection.Description,
			&metadataJSON, &collection.DocumentCount, &collection.CreatedAt, &collection.UpdatedAt); err != nil {
			return nil, err
		}

		if metadataJSON != "" {
			json.Unmarshal([]byte(metadataJSON), &collection.Metadata)
		}
		collections = append(collections, collection)
	}

	return collections, rows.Err()
}

// Update updates a collection
func (r *CollectionRepository) Update(collection *domain.Collection) error {
	collection.UpdatedAt = time.Now()
	metadataJSON, _ := json.Marshal(collection.Metadata)

	result, err := r.db.Exec(`
		UPDATE collections SET name = ?, description = ?, metadata = ?, updated_at = ?
		WHERE id = ?
	`, collection.Name, collection.Description, string(metadataJSON),
		collection.UpdatedAt, collection.ID)

	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("collection not found: %s", collection.ID)
	}

	return nil
}

// Delete deletes a collection
func (r *CollectionRepository) Delete(id string) error {
	result, err := r.db.Exec(`DELETE FROM collections WHERE id = ?`, id)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("collection not found: %s", id)
	}

	return nil
}

// UpdateDocumentCount updates the document count for a collection
func (r *CollectionRepository) UpdateDocumentCount(id string, delta int) error {
	_, err := r.db.Exec(`
		UPDATE collections SET document_count = document_count + ?, updated_at = ?
		WHERE id = ?
	`, delta, time.Now(), id)
	return err
}
