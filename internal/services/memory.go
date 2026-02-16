package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/adk/memory"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// Ensure SQLiteMemoryService implements memory.Service
var _ memory.Service = (*SQLiteMemoryService)(nil)

// SQLiteMemoryService implements memory.Service using SQLite.
type SQLiteMemoryService struct {
	DB *sql.DB
}

// NewSQLiteMemoryService creates a new SQLiteMemoryService.
func NewSQLiteMemoryService(db *sql.DB) *SQLiteMemoryService {
	return &SQLiteMemoryService{DB: db}
}

// AddSession adds a session's relevant content to the memory.
func (s *SQLiteMemoryService) AddSession(ctx context.Context, sess session.Session) error {
	for i := 0; i < sess.Events().Len(); i++ {
		evt := sess.Events().At(i)
		if evt.Content == nil {
			continue
		}

		// Extract text from parts for searchability
		var textContent strings.Builder
		for _, part := range evt.Content.Parts {
			if part.Text != "" {
				textContent.WriteString(part.Text)
				textContent.WriteString(" ")
			}
		}

		content := strings.TrimSpace(textContent.String())
		if content == "" {
			continue
		}

		// Marshal the full genai.Content for the entry
		contentJSON, err := json.Marshal(evt.Content)
		if err != nil {
			return fmt.Errorf("failed to marshal content: %w", err)
		}

		metadata := map[string]any{
			"author": evt.Author,
		}
		metadataJSON, _ := json.Marshal(metadata)

		// Store in memories table with scoping
		_, err = s.DB.ExecContext(ctx,
			"INSERT INTO memories (session_id, user_id, app_name, content, raw_content, metadata, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			sess.ID(), sess.UserID(), sess.AppName(), content, string(contentJSON), string(metadataJSON), evt.Timestamp,
		)
		if err != nil {
			return fmt.Errorf("failed to insert memory: %w", err)
		}
	}
	return nil
}

// Search searches the memory for relevant entries.
func (s *SQLiteMemoryService) Search(ctx context.Context, req *memory.SearchRequest) (*memory.SearchResponse, error) {
	// Filter by AppName to keep it scoped. UserID could also be used.
	query := `
		SELECT raw_content, metadata, created_at 
		FROM memories 
		WHERE content LIKE ? AND app_name = ? AND user_id = ?
		ORDER BY created_at DESC LIMIT 10`

	rows, err := s.DB.QueryContext(ctx, query, "%"+req.Query+"%", req.AppName, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to search memories: %w", err)
	}
	defer rows.Close()

	var memories []memory.Entry
	for rows.Next() {
		var rawContent, metadataJSON []byte
		var createdAt time.Time

		if err := rows.Scan(&rawContent, &metadataJSON, &createdAt); err != nil {
			return nil, err
		}

		var genContent genai.Content
		if err := json.Unmarshal(rawContent, &genContent); err != nil {
			continue
		}

		var metadata map[string]any
		json.Unmarshal(metadataJSON, &metadata)

		author, _ := metadata["author"].(string)

		memories = append(memories, memory.Entry{
			Content:   &genContent,
			Author:    author,
			Timestamp: createdAt,
		})
	}

	return &memory.SearchResponse{Memories: memories}, nil
}

// Deprecated: Use session.State with user: prefix
func (s *SQLiteMemoryService) SetPreference(key, value string) error {
	_, err := s.DB.Exec(
		"INSERT INTO kv_store (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = CURRENT_TIMESTAMP",
		key, value, value,
	)
	return err
}

// Deprecated: Use session.State with user: prefix
func (s *SQLiteMemoryService) GetPreference(key string) (string, error) {
	var value string
	err := s.DB.QueryRow("SELECT value FROM kv_store WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}
