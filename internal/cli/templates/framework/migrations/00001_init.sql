-- +goose Up
CREATE TABLE IF NOT EXISTS gin_kit_health (id INTEGER PRIMARY KEY);

-- +goose Down
DROP TABLE IF EXISTS gin_kit_health;
