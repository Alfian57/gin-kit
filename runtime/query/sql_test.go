package query

import (
	"reflect"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func fullResult() Result {
	return Result{
		Filters: []FilterValue{
			{Name: "completed", Column: "completed", Op: OpEq, Values: []string{"true"}},
			{Name: "title", Column: "title", Op: OpLike, Values: []string{"rep"}},
			{Name: "priority", Column: "priority", Op: OpIn, Values: []string{"1", "2"}},
			{Name: "created_at", Column: "created_at", Op: OpGte, Values: []string{"2026-01-01"}},
		},
		Sorts:   []SortValue{{Column: "created_at", Desc: true}, {Column: "title"}},
		Page:    2,
		PerPage: 10,
	}
}

func TestBuildSQLGolden(t *testing.T) {
	statement, args := fullResult().BuildSQL("SELECT id FROM tasks")
	wantSQL := "SELECT id FROM tasks WHERE completed = ? AND title LIKE ? ESCAPE '!' " +
		"AND priority IN (?, ?) AND created_at >= ? ORDER BY created_at DESC, title ASC LIMIT ? OFFSET ?"
	wantArgs := []any{"true", "%rep%", "1", "2", "2026-01-01", 10, 10}
	if statement != wantSQL {
		t.Fatalf("sql:\n got %q\nwant %q", statement, wantSQL)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args:\n got %#v\nwant %#v", args, wantArgs)
	}
}

func TestBuildCountSQLHasNoOrderOrLimit(t *testing.T) {
	statement, args := fullResult().BuildCountSQL("SELECT COUNT(*) FROM tasks")
	want := "SELECT COUNT(*) FROM tasks WHERE completed = ? AND title LIKE ? ESCAPE '!' " +
		"AND priority IN (?, ?) AND created_at >= ?"
	if statement != want || len(args) != 5 {
		t.Fatalf("count sql: %q args=%d", statement, len(args))
	}
}

func TestBuildSQLWithoutFiltersOrSorts(t *testing.T) {
	statement, args := (Result{Page: 1, PerPage: 25}).BuildSQL("SELECT id FROM tasks")
	if statement != "SELECT id FROM tasks LIMIT ? OFFSET ?" || !reflect.DeepEqual(args, []any{25, 0}) {
		t.Fatalf("sql: %q args=%#v", statement, args)
	}
}

func TestLikeEscaping(t *testing.T) {
	for input, want := range map[string]string{
		"50%":     "%50!%%",
		"a_b":     "%a!_b%",
		"loud!":   "%loud!!%",
		"%_!":     "%!%!_!!%",
		"regular": "%regular%",
	} {
		result := Result{Filters: []FilterValue{{Column: "title", Op: OpLike, Values: []string{input}}}}
		_, args := result.WhereSQL()
		if args[0] != want {
			t.Fatalf("escape(%q) = %q, want %q", input, args[0], want)
		}
	}
}

func TestBuildSQLRoundTripsThroughSQLX(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE tasks (id INTEGER, title TEXT, completed BOOLEAN)`); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		id        int
		title     string
		completed bool
	}{
		{1, "monthly report", true},
		{2, "50%_off sale", true},
		{3, "monthly report", false},
	} {
		if _, err := db.Exec(`INSERT INTO tasks VALUES (?, ?, ?)`, row.id, row.title, row.completed); err != nil {
			t.Fatal(err)
		}
	}

	result := Result{
		Filters: []FilterValue{
			{Column: "completed", Op: OpEq, Values: []string{"true"}, Bool: true},
			{Column: "title", Op: OpLike, Values: []string{"50%_off"}},
		},
		Page: 1, PerPage: 10,
	}
	statement, args := result.BuildSQL("SELECT id FROM tasks")
	var ids []int
	if err := db.Select(&ids, db.Rebind(statement), args...); err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != 2 {
		t.Fatalf("escaped LIKE round trip returned %v", ids)
	}

	countStatement, countArgs := result.BuildCountSQL("SELECT COUNT(*) FROM tasks")
	var total int64
	if err := db.Get(&total, db.Rebind(countStatement), countArgs...); err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("count = %d", total)
	}
}
