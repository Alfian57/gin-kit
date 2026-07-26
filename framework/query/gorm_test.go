package query

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type task struct {
	ID        int
	Title     string
	Completed bool
	CreatedAt time.Time
}

func seededGORM(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&task{}); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for index := 1; index <= 30; index++ {
		row := task{
			ID:        index,
			Title:     fmt.Sprintf("task %02d", index),
			Completed: index%2 == 0,
			// Groups of three rows share one timestamp so cursor walks need
			// the id tiebreak.
			CreatedAt: base.Add(time.Duration((index-1)/3) * time.Minute),
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestApplyGORMFiltersSortsAndPaginates(t *testing.T) {
	db := seededGORM(t)
	result := Result{
		Filters: []FilterValue{{Column: "completed", Op: OpEq, Values: []string{"true"}, Bool: true}},
		Sorts:   []SortValue{{Column: "id", Desc: true}},
		Page:    2,
		PerPage: 5,
	}
	var rows []task
	if err := result.ApplyGORM(db.Model(&task{})).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 || rows[0].ID != 20 || rows[4].ID != 12 {
		t.Fatalf("unexpected page: %+v", rows)
	}

	total, err := result.CountGORM(db.Model(&task{}))
	if err != nil {
		t.Fatal(err)
	}
	if total != 15 {
		t.Fatalf("count = %d, want 15 (unaffected by pagination)", total)
	}
}

// walkCursorGORM pages through the fixture with Parse + ApplyCursorGORM +
// NextCursor until the walk ends and returns the visited ids in order.
func walkCursorGORM(t *testing.T, db *gorm.DB, options Options, key func(task) (string, string)) []int {
	t.Helper()
	var seen []int
	next := ""
	for page := 0; ; page++ {
		if page > 10 {
			t.Fatal("cursor walk did not terminate")
		}
		raw := ""
		if next != "" {
			raw = "cursor=" + next
		}
		result, err := Parse(testContext(t, raw), options)
		if err != nil {
			t.Fatal(err)
		}
		var rows []task
		if err := result.ApplyCursorGORM(db.Model(&task{})).Find(&rows).Error; err != nil {
			t.Fatal(err)
		}
		rows, next = NextCursor(result, rows, key)
		for _, row := range rows {
			seen = append(seen, row.ID)
		}
		if next == "" {
			return seen
		}
		if len(rows) != result.PerPage {
			t.Fatalf("page %d has %d rows but produced a next cursor", page, len(rows))
		}
	}
}

func TestApplyCursorGORMWalksByID(t *testing.T) {
	seen := walkCursorGORM(t, seededGORM(t), Options{
		AllowedSorts:   []Sort{SortBy("id")},
		DefaultSort:    "-id",
		CursorSort:     "id",
		DefaultPerPage: 10,
	}, func(row task) (string, string) {
		id := strconv.Itoa(row.ID)
		return id, id
	})
	if len(seen) != 30 {
		t.Fatalf("walk visited %d rows: %v", len(seen), seen)
	}
	for index, id := range seen {
		if id != 30-index {
			t.Fatalf("no-overlap/no-gap violated at %d: %v", index, seen)
		}
	}
}

// sqliteTimeLayout pins how the sqlite driver stores time.Time values: as
// text in the driver's timestamp format. Cursor values bind as strings and
// compare against that stored text, so the key function must serialize with
// the same layout for the equality tiebreak to match.
const sqliteTimeLayout = "2006-01-02 15:04:05.999999999-07:00"

func TestApplyCursorGORMWalksByTimeWithTiebreak(t *testing.T) {
	seen := walkCursorGORM(t, seededGORM(t), Options{
		AllowedSorts:   []Sort{SortBy("created_at")},
		DefaultSort:    "created_at",
		CursorSort:     "created_at",
		DefaultPerPage: 7,
	}, func(row task) (string, string) {
		return row.CreatedAt.UTC().Format(sqliteTimeLayout), strconv.Itoa(row.ID)
	})
	if len(seen) != 30 {
		t.Fatalf("walk visited %d rows: %v", len(seen), seen)
	}
	// Ascending created_at with the id tiebreak visits ids 1..30 in order:
	// page boundaries fall inside groups of equal timestamps.
	for index, id := range seen {
		if id != index+1 {
			t.Fatalf("no-overlap/no-gap violated at %d: %v", index, seen)
		}
	}
}

func TestApplyGORMPartialAndInFilters(t *testing.T) {
	db := seededGORM(t)
	result := Result{
		Filters: []FilterValue{
			{Column: "title", Op: OpLike, Values: []string{"task 0"}},
			{Column: "id", Op: OpIn, Values: []string{"1", "2", "3", "25"}},
		},
		Page:    1,
		PerPage: 25,
	}
	var rows []task
	if err := result.ApplyGORM(db.Model(&task{})).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected ids 1-3 (title match), got %+v", rows)
	}
}
