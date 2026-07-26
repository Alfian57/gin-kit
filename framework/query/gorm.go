package query

import (
	"gorm.io/gorm"
)

// FiltersGORM applies only the filters, for use in count queries or custom
// chains.
func (r Result) FiltersGORM(db *gorm.DB) *gorm.DB {
	for _, filter := range r.Filters {
		switch filter.Op {
		case OpEq:
			db = db.Where(filter.Column+" = ?", filter.arg(filter.Values[0]))
		case OpLike:
			db = db.Where(filter.Column+" LIKE ? ESCAPE '!'", "%"+likeEscaper.Replace(filter.Values[0])+"%")
		case OpIn:
			args := make([]any, len(filter.Values))
			for index, value := range filter.Values {
				args[index] = filter.arg(value)
			}
			db = db.Where(filter.Column+" IN ?", args)
		case OpGte, OpLte, OpGt, OpLt:
			db = db.Where(filter.Column+" "+comparison(filter.Op)+" ?", filter.arg(filter.Values[0]))
		}
	}
	return db
}

// ApplyGORM applies filters, sorts, and pagination to a GORM chain.
func (r Result) ApplyGORM(db *gorm.DB) *gorm.DB {
	db = r.FiltersGORM(db)
	for _, sort := range r.Sorts {
		direction := " ASC"
		if sort.Desc {
			direction = " DESC"
		}
		db = db.Order(sort.Column + direction)
	}
	return db.Limit(r.PerPage).Offset(r.Offset())
}

// CountGORM counts the rows matching the filters in an isolated session, so
// the caller's chain can still receive ApplyGORM afterwards.
func (r Result) CountGORM(db *gorm.DB) (int64, error) {
	var total int64
	err := r.FiltersGORM(db.Session(&gorm.Session{})).Count(&total).Error
	return total, err
}
