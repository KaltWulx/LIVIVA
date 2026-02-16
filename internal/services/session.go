package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"iter"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/adk/session"
)

// ensure DatabaseSessionService implements session.Service
var _ session.Service = (*DatabaseSessionService)(nil)

// DatabaseSessionService implements session.Service using SQLite.
type DatabaseSessionService struct {
	db *sql.DB
}

// NewDatabaseSessionService creates a new DatabaseSessionService.
func NewDatabaseSessionService(db *sql.DB) *DatabaseSessionService {
	return &DatabaseSessionService{db: db}
}

// Create creates a new session.
func (s *DatabaseSessionService) Create(ctx context.Context, req *session.CreateRequest) (*session.CreateResponse, error) {
	if req.SessionID == "" {
		req.SessionID = uuid.NewString()
	}

	// Prepare initial state
	stateJSON, err := json.Marshal(req.State)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal initial state: %w", err)
	}

	now := time.Now()
	_, err = s.db.ExecContext(ctx,
		"INSERT INTO sessions (session_id, app_name, user_id, state, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		req.SessionID, req.AppName, req.UserID, string(stateJSON), now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Construct the session object
	// For a fresh session, we use the provided state and empty events
	sess := &SQLiteSession{
		id:         req.SessionID,
		appName:    req.AppName,
		userID:     req.UserID,
		lastUpdate: now,
		state:      NewSQLiteState(s.db, req.SessionID, req.UserID, req.AppName, ReqStateToMap(req.State)),
		events:     &SQLiteEvents{events: []*session.Event{}},
	}

	return &session.CreateResponse{Session: sess}, nil
}

// Get retrieves a session by ID.
func (s *DatabaseSessionService) Get(ctx context.Context, req *session.GetRequest) (*session.GetResponse, error) {
	var appName, userID string
	var stateJSON []byte
	var updatedAt time.Time

	err := s.db.QueryRowContext(ctx,
		"SELECT app_name, user_id, state, updated_at FROM sessions WHERE session_id = ?",
		req.SessionID,
	).Scan(&appName, &userID, &stateJSON, &updatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found: %s", req.SessionID)
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Unmarshal session-scoped state
	sessionState := make(map[string]any)
	if len(stateJSON) > 0 {
		if err := json.Unmarshal(stateJSON, &sessionState); err != nil {
			return nil, fmt.Errorf("failed to unmarshal session state: %w", err)
		}
	}

	// Load history (Events)
	// Optimization: Enforce a default context window to prevent LLM confusion and token waste
	limit := req.NumRecentEvents
	if limit <= 0 {
		limit = 8 // Default to last 8 events for immediate conversational flow
	}
	events, err := s.loadEvents(ctx, req.SessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to load events: %w", err)
	}

	sess := &SQLiteSession{
		id:         req.SessionID,
		appName:    appName,
		userID:     userID,
		lastUpdate: updatedAt,
		state:      NewSQLiteState(s.db, req.SessionID, userID, appName, sessionState),
		events:     &SQLiteEvents{events: events},
	}

	return &session.GetResponse{Session: sess}, nil
}

// loadEvents loads events for a session from the database.
func (s *DatabaseSessionService) loadEvents(ctx context.Context, sessionID string, limit int) ([]*session.Event, error) {
	query := "SELECT content FROM events WHERE session_id = ? ORDER BY timestamp ASC"
	var rows *sql.Rows
	var err error

	if limit > 0 {
		// Optimization: If we only want recent events, we should order by DESC, limit, then reverse.
		// Or we can use a subquery.
		// "SELECT content FROM (SELECT content, timestamp FROM events WHERE session_id = ? ORDER BY timestamp DESC LIMIT ?) ORDER BY timestamp ASC"
		query = "SELECT content FROM (SELECT content, timestamp FROM events WHERE session_id = ? ORDER BY timestamp DESC LIMIT ?) ORDER BY timestamp ASC"
		rows, err = s.db.QueryContext(ctx, query, sessionID, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, query, sessionID)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*session.Event
	for rows.Next() {
		var contentJSON []byte
		if err := rows.Scan(&contentJSON); err != nil {
			return nil, err
		}

		var evt session.Event
		if err := json.Unmarshal(contentJSON, &evt); err != nil {
			// Skip malformed events or log error
			continue
		}
		events = append(events, &evt)
	}
	return events, nil
}

// List sessions (Not implemented)
func (s *DatabaseSessionService) List(ctx context.Context, req *session.ListRequest) (*session.ListResponse, error) {
	return &session.ListResponse{}, nil
}

// Delete session
func (s *DatabaseSessionService) Delete(ctx context.Context, req *session.DeleteRequest) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE session_id = ?", req.SessionID)
	s.db.ExecContext(ctx, "DELETE FROM events WHERE session_id = ?", req.SessionID)
	return err
}

// AppendEvent adds an event to the session history and updates state.
func (s *DatabaseSessionService) AppendEvent(ctx context.Context, sess session.Session, evt *session.Event) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Insert Event
	contentJSON, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		"INSERT INTO events (session_id, content, timestamp) VALUES (?, ?, ?)",
		sess.ID(), contentJSON, evt.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	// 2. Update Session State
	// We need to merge the state delta into the stored state.
	// Since we implement splitting logic, we need to process the delta.
	// However, the `sess` object passed here might already have the updated state in memory if it was modified in-place?
	// ADK docs say AppendEvent "updates state".
	// Actually, `EventActions.StateDelta` contains the changes.

	if evt.Actions.StateDelta != nil {
		// Load current session state blob
		var stateJSON []byte
		if err := tx.QueryRowContext(ctx, "SELECT state FROM sessions WHERE session_id = ?", sess.ID()).Scan(&stateJSON); err != nil {
			return err
		}
		currentState := make(map[string]any)
		if len(stateJSON) > 0 {
			json.Unmarshal(stateJSON, &currentState)
		}

		for k, v := range evt.Actions.StateDelta {
			// Handle deletions ? ADK doesn't specify deletion explicitly in delta, usually setting to nil?
			// But `any` can be nil.

			if strings.HasPrefix(k, "user:") {
				// Update user kv_store
				valJSON, _ := json.Marshal(v)
				key := fmt.Sprintf("user:%s:%s", sess.UserID(), strings.TrimPrefix(k, "user:"))
				_, err = tx.ExecContext(ctx, "INSERT INTO kv_store (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = CURRENT_TIMESTAMP", key, string(valJSON), string(valJSON))
				if err != nil {
					return err
				}
			} else if strings.HasPrefix(k, "app:") {
				// Update app kv_store
				valJSON, _ := json.Marshal(v)
				key := fmt.Sprintf("app:%s:%s", sess.AppName(), strings.TrimPrefix(k, "app:"))
				_, err = tx.ExecContext(ctx, "INSERT INTO kv_store (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = CURRENT_TIMESTAMP", key, string(valJSON), string(valJSON))
				if err != nil {
					return err
				}
			} else if strings.HasPrefix(k, "temp:") {
				// Ignore temp state
				continue
			} else {
				// Session state
				currentState[k] = v
			}
		}

		newStateJSON, _ := json.Marshal(currentState)
		_, err = tx.ExecContext(ctx, "UPDATE sessions SET state = ?, updated_at = ? WHERE session_id = ?", string(newStateJSON), time.Now(), sess.ID())
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// --- SQLite Implementations of Interfaces ---

type SQLiteSession struct {
	id         string
	appName    string
	userID     string
	lastUpdate time.Time
	state      *SQLiteState
	events     *SQLiteEvents
}

func (s *SQLiteSession) ID() string                { return s.id }
func (s *SQLiteSession) AppName() string           { return s.appName }
func (s *SQLiteSession) UserID() string            { return s.userID }
func (s *SQLiteSession) State() session.State      { return s.state }
func (s *SQLiteSession) Events() session.Events    { return s.events }
func (s *SQLiteSession) LastUpdateTime() time.Time { return s.lastUpdate }

type SQLiteState struct {
	db        *sql.DB
	sessionID string
	userID    string
	appName   string
	localMap  map[string]any
}

func NewSQLiteState(db *sql.DB, sID, uID, app string, local map[string]any) *SQLiteState {
	if local == nil {
		local = make(map[string]any)
	}
	return &SQLiteState{
		db:        db,
		sessionID: sID,
		userID:    uID,
		appName:   app,
		localMap:  local,
	}
}

func (s *SQLiteState) Get(key string) (any, error) {
	// 1. Check local session state
	if val, ok := s.localMap[key]; ok {
		return val, nil
	}

	// 2. Check User State
	if strings.HasPrefix(key, "user:") {
		dbKey := fmt.Sprintf("user:%s:%s", s.userID, strings.TrimPrefix(key, "user:"))
		return s.getFromKV(dbKey)
	}

	// 3. Check App State
	if strings.HasPrefix(key, "app:") {
		dbKey := fmt.Sprintf("app:%s:%s", s.appName, strings.TrimPrefix(key, "app:"))
		return s.getFromKV(dbKey)
	}

	return nil, session.ErrStateKeyNotExist
}

func (s *SQLiteState) getFromKV(key string) (any, error) {
	var valJSON string
	err := s.db.QueryRow("SELECT value FROM kv_store WHERE key = ?", key).Scan(&valJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, session.ErrStateKeyNotExist
		}
		return nil, err
	}
	var val any
	if err := json.Unmarshal([]byte(valJSON), &val); err != nil {
		return nil, err
	}
	return val, nil
}

func (s *SQLiteState) Set(key string, value any) error {
	// ADK usually updates state via AppendEvent, but Set might be called directly??
	// If called directly, we should persist immediately?
	// The `session.State` interface has Set.
	// For now, we update in-memory request-scoped map AND persist if it's user/app
	// But `Set` is synchronous.
	// This implementation is tricky without Transaction context.
	// We'll simplisticly update DB for global keys, and local map for session keys.
	// NOTE: Real ADK flow uses AppendEvent for everything.

	if strings.HasPrefix(key, "user:") {
		valJSON, _ := json.Marshal(value)
		dbKey := fmt.Sprintf("user:%s:%s", s.userID, strings.TrimPrefix(key, "user:"))
		_, err := s.db.Exec("INSERT INTO kv_store (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = CURRENT_TIMESTAMP", dbKey, string(valJSON), string(valJSON))
		return err
	}
	if strings.HasPrefix(key, "app:") {
		valJSON, _ := json.Marshal(value)
		dbKey := fmt.Sprintf("app:%s:%s", s.appName, strings.TrimPrefix(key, "app:"))
		_, err := s.db.Exec("INSERT INTO kv_store (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = CURRENT_TIMESTAMP", dbKey, string(valJSON), string(valJSON))
		return err
	}

	s.localMap[key] = value
	// For session keys, we don't persist on Set because it requires updating the full JSON blob.
	// We assume AppendEvent will be called later to persist the session state change?
	// actually Set is rarely used outside of initialization or tests in ADK.
	return nil
}

func (s *SQLiteState) All() iter.Seq2[string, any] {
	return func(yield func(string, any) bool) {
		for k, v := range s.localMap {
			if !yield(k, v) {
				return
			}
		}
		// TODO: Iterate user/app keys from DB? Not performant.
	}
}

type SQLiteEvents struct {
	events []*session.Event
}

func (e *SQLiteEvents) All() iter.Seq[*session.Event] {
	return func(yield func(*session.Event) bool) {
		for _, evt := range e.events {
			if !yield(evt) {
				return
			}
		}
	}
}

func (e *SQLiteEvents) Len() int {
	return len(e.events)
}

func (e *SQLiteEvents) At(i int) *session.Event {
	return e.events[i]
}

// Helper to convert map[string]any to map[string]any (identity, just for typing)
func ReqStateToMap(m map[string]any) map[string]any {
	if m == nil {
		return make(map[string]any)
	}
	return m
}
