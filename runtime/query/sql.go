package query

import (
	"strings"
)

// likeEscaper escapes LIKE wildcards with '!', the one escape character that
// behaves identically across MySQL, MariaDB, PostgreSQL, and SQLite when
// declared with ESCAPE '!'.
var likeEscaper = strings.NewReplacer("!", "!!", "%", "!%", "_", "!_")

// WhereSQL renders the filters as a parameterized SQL condition using '?'
// placeholders, e.g. "completed = ? AND title LIKE ? ESCAPE '!'". sqlx callers
// pass the final statement through Rebind for their dialect. The clause and
// args are empty when no filters are present.
func (r Result) WhereSQL() (string, []any) {
	if len(r.Filters) == 0 {
		return "", nil
	}
	conditions := make([]string, 0, len(r.Filters))
	var args []any
	for _, filter := range r.Filters {
		switch filter.Op {
		case OpEq:
			conditions = append(conditions, filter.Column+" = ?")
			args = append(args, filter.arg(filter.Values[0]))
		case OpLike:
			conditions = append(conditions, filter.Column+" LIKE ? ESCAPE '!'")
			args = append(args, "%"+likeEscaper.Replace(filter.Values[0])+"%")
		case OpIn:
			placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(filter.Values)), ", ")
			conditions = append(conditions, filter.Column+" IN ("+placeholders+")")
			for _, value := range filter.Values {
				args = append(args, filter.arg(value))
			}
		case OpGte, OpLte, OpGt, OpLt:
			conditions = append(conditions, filter.Column+" "+comparison(filter.Op)+" ?")
			args = append(args, filter.arg(filter.Values[0]))
		}
	}
	return strings.Join(conditions, " AND "), args
}

// OrderSQL renders "ORDER BY ..." for the accepted sorts, or "".
func (r Result) OrderSQL() string {
	if len(r.Sorts) == 0 {
		return ""
	}
	fields := make([]string, 0, len(r.Sorts))
	for _, sort := range r.Sorts {
		direction := " ASC"
		if sort.Desc {
			direction = " DESC"
		}
		fields = append(fields, sort.Column+direction)
	}
	return "ORDER BY " + strings.Join(fields, ", ")
}

// BuildSQL appends WHERE, ORDER BY, LIMIT, and OFFSET to a base statement such
// as "SELECT id, title FROM tasks".
func (r Result) BuildSQL(base string) (string, []any) {
	statement, args := r.appendWhere(base)
	if order := r.OrderSQL(); order != "" {
		statement += " " + order
	}
	statement += " LIMIT ? OFFSET ?"
	args = append(args, r.PerPage, r.Offset())
	return statement, args
}

// BuildCountSQL appends only WHERE to a base statement such as
// "SELECT COUNT(*) FROM tasks".
func (r Result) BuildCountSQL(base string) (string, []any) {
	return r.appendWhere(base)
}

func (r Result) appendWhere(base string) (string, []any) {
	where, args := r.WhereSQL()
	if where != "" {
		base += " WHERE " + where
	}
	return base, args
}

func comparison(op Op) string {
	switch op {
	case OpGte:
		return ">="
	case OpLte:
		return "<="
	case OpGt:
		return ">"
	default:
		return "<"
	}
}
