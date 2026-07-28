-- +goose Up
CREATE TABLE IF NOT EXISTS oauth_identities (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    provider VARCHAR(32) NOT NULL,
    provider_subject VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    CONSTRAINT oauth_identities_provider_subject_unique UNIQUE (provider, provider_subject),
    CONSTRAINT oauth_identities_user_fk FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);
CREATE INDEX oauth_identities_user_id_idx ON oauth_identities (user_id);

-- +goose Down
DROP TABLE IF EXISTS oauth_identities;
