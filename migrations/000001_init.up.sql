-- Initial schema for jellyfin-share-backend

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Shares table
CREATE TABLE shares (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    public_token VARCHAR(64) NOT NULL UNIQUE,
    jellyfin_item_id VARCHAR(64) NOT NULL,
    jellyfin_user_id VARCHAR(64) NOT NULL,
    title VARCHAR(512) NOT NULL,
    overview TEXT,
    runtime_seconds INTEGER,
    poster_path VARCHAR(512),
    backdrop_path VARCHAR(512),
    item_type VARCHAR(64) NOT NULL DEFAULT 'Movie',
    max_total_plays INTEGER,
    max_concurrent_viewers INTEGER,
    total_plays INTEGER NOT NULL DEFAULT 0,
    current_concurrent_viewers INTEGER NOT NULL DEFAULT 0,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    password_hash VARCHAR(256),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMP WITH TIME ZONE,
    last_activity_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_shares_public_token ON shares(public_token);
CREATE INDEX idx_shares_jellyfin_user_id ON shares(jellyfin_user_id);
CREATE INDEX idx_shares_expires_at ON shares(expires_at);
CREATE INDEX idx_shares_jellyfin_item_id ON shares(jellyfin_item_id);

-- Share sessions table
CREATE TABLE share_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    share_id UUID NOT NULL REFERENCES shares(id) ON DELETE CASCADE,
    session_token VARCHAR(128) NOT NULL UNIQUE,
    client_ip_hash VARCHAR(64),
    user_agent VARCHAR(512),
    started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_heartbeat_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMP WITH TIME ZONE,
    terminated_reason VARCHAR(32),
    last_position_secs INTEGER
);

CREATE INDEX idx_share_sessions_share_id ON share_sessions(share_id);
CREATE INDEX idx_share_sessions_session_token ON share_sessions(session_token);
CREATE INDEX idx_share_sessions_last_heartbeat ON share_sessions(last_heartbeat_at);

-- Audit log table for tracking events
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_type VARCHAR(64) NOT NULL,
    share_id UUID REFERENCES shares(id) ON DELETE SET NULL,
    session_id UUID REFERENCES share_sessions(id) ON DELETE SET NULL,
    jellyfin_user_id VARCHAR(64),
    client_ip_hash VARCHAR(64),
    details JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_event_type ON audit_logs(event_type);
CREATE INDEX idx_audit_logs_share_id ON audit_logs(share_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
