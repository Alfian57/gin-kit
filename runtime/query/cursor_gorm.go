package query

import (
	"gorm.io/gorm"
)

// ApplyCursorGORM applies filters, the keyset predicate, the forced cursor
// ordering, and the probe-row limit (CursorLimit — never an offset) to a
// GORM chain. Trim the probe row with NextCursor before responding.
func (r Result) ApplyCursorGORM(db *gorm.DB) *gorm.DB {
	db = r.FiltersGORM(db)
	if keyset, args := r.keysetSQL(); keyset != "" {
		db = db.Where(keyset, args...)
	}
	for _, sort := range r.Sorts {
		direction := " ASC"
		if sort.Desc {
			direction = " DESC"
		}
		db = db.Order(sort.Column + direction)
	}
	return db.Limit(r.CursorLimit())
}
