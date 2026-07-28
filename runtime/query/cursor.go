package query

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// Cursor is a decoded keyset-pagination position: the sort value and id of
// the last row on the previous page. Both bind as strings, like Compare
// filter values.
type Cursor struct {
	// Value store data used by this type.
	Value string
	// ID store data used by this type.
	ID string
}

// cursorPayload is the JSON wire shape of a cursor token.
type cursorPayload struct {
	// Value store data used by this type.
	Value string `json:"v"`
	// ID store data used by this type.
	ID string `json:"id"`
}

// EncodeCursor renders the cursor as an opaque, unpadded base64url token.
func EncodeCursor(c Cursor) string {
	payload, _ := json.Marshal(cursorPayload{Value: c.Value, ID: c.ID})
	return base64.RawURLEncoding.EncodeToString(payload)
}

// DecodeCursor parses a token produced by EncodeCursor.
func DecodeCursor(raw string) (Cursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return Cursor{}, fmt.Errorf("cursor is not valid base64url: %w", err)
	}
	var payload cursorPayload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return Cursor{}, fmt.Errorf("cursor payload is invalid: %w", err)
	}
	return Cursor{Value: payload.Value, ID: payload.ID}, nil
}

// CursorMeta is the pagination metadata for cursor mode. NextCursor is null
// on the last page.
type CursorMeta struct {
	// NextCursor store data used by this type.
	NextCursor *string `json:"next_cursor"`
	// PerPage store data used by this type.
	PerPage int `json:"per_page"`
}

// CursorMeta builds cursor pagination metadata for httpx.List; next == ""
// marks the last page and serializes next_cursor as null.
func (r Result) CursorMeta(next string) CursorMeta {
	meta := CursorMeta{PerPage: r.PerPage}
	if next != "" {
		meta.NextCursor = &next
	}
	return meta
}

// CursorLimit returns the row limit for cursor mode: PerPage plus one probe
// row that detects whether a next page exists. Trim the probe row with
// NextCursor before responding.
func (r Result) CursorLimit() int { return r.PerPage + 1 }

// keysetSQL renders the keyset predicate for the decoded cursor with '?'
// placeholders, e.g. "(created_at < ? OR (created_at = ? AND id < ?))" for a
// descending sort. It is empty on the first page.
func (r Result) keysetSQL() (string, []any) {
	if r.Cursor == nil || len(r.Sorts) == 0 {
		return "", nil
	}
	op := ">"
	if r.Sorts[0].Desc {
		op = "<"
	}
	column := r.Sorts[0].Column
	predicate := "(" + column + " " + op + " ? OR (" + column + " = ? AND id " + op + " ?))"
	return predicate, []any{r.Cursor.Value, r.Cursor.Value, r.Cursor.ID}
}

// BuildCursorSQL appends WHERE (filters plus the keyset predicate), ORDER BY,
// and "LIMIT ?" bound to CursorLimit — never OFFSET — to a base statement
// such as "SELECT id, title FROM tasks". sqlx callers pass the final
// statement through Rebind for their dialect.
func (r Result) BuildCursorSQL(base string) (string, []any) {
	where, args := r.WhereSQL()
	if keyset, keysetArgs := r.keysetSQL(); keyset != "" {
		if where != "" {
			where += " AND " + keyset
		} else {
			where = keyset
		}
		args = append(args, keysetArgs...)
	}
	if where != "" {
		base += " WHERE " + where
	}
	if order := r.OrderSQL(); order != "" {
		base += " " + order
	}
	base += " LIMIT ?"
	args = append(args, r.CursorLimit())
	return base, args
}

// NextCursor trims the probe row fetched by CursorLimit and derives the next
// request's cursor from the last visible item. It returns the visible page
// and the encoded next cursor, or "" when the page was not full.
func NextCursor[T any](r Result, items []T, key func(T) (value, id string)) ([]T, string) {
	if len(items) <= r.PerPage {
		return items, ""
	}
	items = items[:r.PerPage]
	value, id := key(items[len(items)-1])
	return items, EncodeCursor(Cursor{Value: value, ID: id})
}
