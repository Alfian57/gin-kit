-- +goose Up
CREATE TABLE IF NOT EXISTS ginkit_health (id INTEGER PRIMARY KEY);

-- +goose Down
DROP TABLE IF EXISTS ginkit_health;
