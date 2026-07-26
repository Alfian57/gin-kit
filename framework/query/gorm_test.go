package query

import (
	"fmt"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type task struct {
	ID        int
	Title     string
	Completed bool
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
	for index := 1; index <= 30; index++ {
		row := task{ID: index, Title: fmt.Sprintf("task %02d", index), Completed: index%2 == 0}
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
