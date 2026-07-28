package factory

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// user defines an implementation type used by this package.
type user struct {
	// ID store data used by this type.
	ID int
	// Email store data used by this type.
	Email string
	// Name store data used by this type.
	Name string
}

// userFactory performs this package operation.
func userFactory() *Factory[user] {
	return Define(func(f *F) user {
		return user{ID: f.Seq(), Email: f.Seqf("user-%d@example.com"), Name: f.Name()}
	})
}

func TestMakeAppliesOverridesInOrder(t *testing.T) {
	made := userFactory().Make(
		func(u *user) { u.Name = "first" },
		func(u *user) { u.Name = "second" },
	)
	if made.Name != "second" {
		t.Fatalf("overrides not applied in order: %q", made.Name)
	}
	if made.Email != "user-1@example.com" {
		t.Fatalf("build function values missing: %+v", made)
	}
}

func TestSequenceIncrementsAcrossMakes(t *testing.T) {
	fa := userFactory()
	first := fa.Make()
	second := fa.Make()
	many := fa.MakeMany(2)
	ids := []int{first.ID, second.ID, many[0].ID, many[1].ID}
	for index, want := range []int{1, 2, 3, 4} {
		if ids[index] != want {
			t.Fatalf("sequence broken: %v", ids)
		}
	}
}

func TestSeededFactoryIsDeterministic(t *testing.T) {
	base := userFactory()
	first := base.Seeded(42).MakeMany(3)
	second := base.Seeded(42).MakeMany(3)
	for index := range first {
		if first[index] != second[index] {
			t.Fatalf("seeded factories diverged at %d: %+v vs %+v", index, first[index], second[index])
		}
	}
}

func TestMakeManyReturnsN(t *testing.T) {
	if got := len(userFactory().MakeMany(5)); got != 5 {
		t.Fatalf("MakeMany(5) returned %d", got)
	}
}

func TestCreatePersistsAndReturnsEntity(t *testing.T) {
	var persisted []user
	persist := func(_ context.Context, u *user) error {
		persisted = append(persisted, *u)
		return nil
	}
	fa := userFactory()
	created, err := fa.Create(context.Background(), persist, func(u *user) { u.Name = "override" })
	if err != nil || created.Name != "override" {
		t.Fatalf("create: %+v err=%v", created, err)
	}
	many, err := fa.CreateMany(context.Background(), 3, persist)
	if err != nil || len(many) != 3 || len(persisted) != 4 {
		t.Fatalf("create many: %d created, %d persisted, err=%v", len(many), len(persisted), err)
	}
}

func TestCreatePropagatesPersistError(t *testing.T) {
	sentinel := errors.New("db offline")
	calls := 0
	persist := func(context.Context, *user) error {
		calls++
		if calls == 2 {
			return sentinel
		}
		return nil
	}
	values, err := userFactory().CreateMany(context.Background(), 5, persist)
	if !errors.Is(err, sentinel) || len(values) != 1 {
		t.Fatalf("expected stop at first failure: %d values, err=%v", len(values), err)
	}
}

func TestConcurrentMakeIsSafe(t *testing.T) {
	fa := userFactory()
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				_ = fa.Make()
			}
		}()
	}
	wg.Wait()
	if next := fa.Make(); next.ID != 401 {
		t.Fatalf("sequence lost under concurrency: %d", next.ID)
	}
}
