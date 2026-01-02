package database

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

type AuditEventType string

const (
	AuditEventShareCreated     AuditEventType = "share_created"
	AuditEventShareRevoked     AuditEventType = "share_revoked"
	AuditEventShareUpdated     AuditEventType = "share_updated"
	AuditEventShareAccessed    AuditEventType = "share_accessed"
	AuditEventPasswordAttempt  AuditEventType = "password_attempt"
	AuditEventPlaybackStarted  AuditEventType = "playback_started"
	AuditEventPlaybackEnded    AuditEventType = "playback_ended"
	AuditEventPlaybackDenied   AuditEventType = "playback_denied"
	AuditEventSessionTimeout   AuditEventType = "session_timeout"
)

type AuditLog struct {
	ID             uuid.UUID              `db:"id" json:"id"`
	EventType      string                 `db:"event_type" json:"eventType"`
	ShareID        *uuid.UUID             `db:"share_id" json:"shareId,omitempty"`
	SessionID      *uuid.UUID             `db:"session_id" json:"sessionId,omitempty"`
	JellyfinUserID *string                `db:"jellyfin_user_id" json:"jellyfinUserId,omitempty"`
	ClientIPHash   *string                `db:"client_ip_hash" json:"clientIpHash,omitempty"`
	Details        map[string]interface{} `json:"details,omitempty"`
	CreatedAt      string                 `db:"created_at" json:"createdAt"`
}

func (db *DB) LogAuditEvent(ctx context.Context, eventType AuditEventType, shareID, sessionID *uuid.UUID, jellyfinUserID, clientIPHash *string, details map[string]interface{}) error {
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		detailsJSON = []byte("{}")
	}

	query := `
		INSERT INTO audit_logs (event_type, share_id, session_id, jellyfin_user_id, client_ip_hash, details)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err = db.ExecContext(ctx, query, string(eventType), shareID, sessionID, jellyfinUserID, clientIPHash, detailsJSON)
	if err != nil {
		return fmt.Errorf("failed to log audit event: %w", err)
	}
	return nil
}

func (db *DB) GetAuditLogs(ctx context.Context, shareID *uuid.UUID, limit, offset int) ([]AuditLog, error) {
	var logs []AuditLog
	var query string
	var args []interface{}

	if shareID != nil {
		query = `SELECT id, event_type, share_id, session_id, jellyfin_user_id, client_ip_hash, created_at
		         FROM audit_logs WHERE share_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		args = []interface{}{shareID, limit, offset}
	} else {
		query = `SELECT id, event_type, share_id, session_id, jellyfin_user_id, client_ip_hash, created_at
		         FROM audit_logs ORDER BY created_at DESC LIMIT $1 OFFSET $2`
		args = []interface{}{limit, offset}
	}

	err := db.SelectContext(ctx, &logs, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit logs: %w", err)
	}
	return logs, nil
}
