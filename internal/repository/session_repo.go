package repository

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/liliang-cn/askdoc/internal/domain"
)

// SessionRepository handles session persistence
type SessionRepository struct {
	db *DB
}

// NewSessionRepository creates a new session repository
func NewSessionRepository(db *DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// Create creates a new session
func (r *SessionRepository) Create(session *domain.Session) error {
	if session.ID == "" {
		session.ID = uuid.New().String()
	}
	now := time.Now()
	session.CreatedAt = now
	session.UpdatedAt = now

	_, err := r.db.Exec(`
		INSERT INTO sessions (id, site_id, created_at, updated_at)
		VALUES (?, ?, ?, ?)
	`, session.ID, session.SiteID, session.CreatedAt, session.UpdatedAt)

	return err
}

// Get retrieves a session by ID
func (r *SessionRepository) Get(id string) (*domain.Session, error) {
	session := &domain.Session{}
	var siteID sql.NullString

	err := r.db.QueryRow(`
		SELECT id, site_id, created_at, updated_at
		FROM sessions WHERE id = ?
	`, id).Scan(&session.ID, &siteID, &session.CreatedAt, &session.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if siteID.Valid {
		session.SiteID = siteID.String
	}

	return session, nil
}

// Update updates a session's updated_at timestamp
func (r *SessionRepository) Update(id string) error {
	_, err := r.db.Exec(`UPDATE sessions SET updated_at = ? WHERE id = ?`, time.Now(), id)
	return err
}

// CreateMessage creates a new message
func (r *SessionRepository) CreateMessage(message *domain.Message) error {
	if message.ID == "" {
		message.ID = uuid.New().String()
	}
	message.CreatedAt = time.Now()

	sourcesJSON, _ := json.Marshal(message.Sources)

	_, err := r.db.Exec(`
		INSERT INTO messages (id, session_id, role, content, sources, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, message.ID, message.SessionID, message.Role, message.Content,
		string(sourcesJSON), message.CreatedAt)

	return err
}

// GetMessages retrieves all messages for a session
func (r *SessionRepository) GetMessages(sessionID string) ([]*domain.Message, error) {
	rows, err := r.db.Query(`
		SELECT id, session_id, role, content, sources, created_at
		FROM messages WHERE session_id = ?
		ORDER BY created_at ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*domain.Message
	for rows.Next() {
		message := &domain.Message{}
		var sourcesJSON sql.NullString

		if err := rows.Scan(&message.ID, &message.SessionID, &message.Role,
			&message.Content, &sourcesJSON, &message.CreatedAt); err != nil {
			return nil, err
		}

		if sourcesJSON.Valid && sourcesJSON.String != "" {
			json.Unmarshal([]byte(sourcesJSON.String), &message.Sources)
		}
		messages = append(messages, message)
	}

	return messages, rows.Err()
}

// CountChats returns the total number of user messages (chats)
func (r *SessionRepository) CountChats() (int, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE role = 'user'`).Scan(&count)
	return count, err
}
