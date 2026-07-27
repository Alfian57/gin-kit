package flags

import (
	"reflect"
	"sync"
	"testing"
)

func TestParse(t *testing.T) {
	set := Parse(" checkout-v2, reports ,, checkout-v2, ")

	if !set.Enabled("checkout-v2") {
		t.Fatal("checkout-v2 is not enabled")
	}
	if !set.Enabled(" reports ") {
		t.Fatal("Enabled must trim the queried name")
	}
	if set.Enabled("") {
		t.Fatal("empty name must not be enabled")
	}
	if got, want := set.Names(), []string{"checkout-v2", "reports"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Names() = %#v; want %#v", got, want)
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv("FLAGS", "new-nav, billing-v2")

	set := FromEnv()

	if !set.Enabled("new-nav") || !set.Enabled("billing-v2") {
		t.Fatalf("FromEnv() did not parse FLAGS: %#v", set.Names())
	}
}

func TestSetEnablesAndDisables(t *testing.T) {
	var set Set

	set.Set("search-v2", true)
	if !set.Enabled("search-v2") {
		t.Fatal("Set(name, true) did not enable the flag")
	}

	set.Set("search-v2", false)
	if set.Enabled("search-v2") {
		t.Fatal("Set(name, false) did not disable the flag")
	}
	if got := set.Names(); len(got) != 0 {
		t.Fatalf("disabled flag remains in Names(): %#v", got)
	}
}

func TestNilSetIsSafeToQuery(t *testing.T) {
	var set *Set

	if set.Enabled("anything") {
		t.Fatal("nil Set reported an enabled flag")
	}
	if got := set.Names(); len(got) != 0 {
		t.Fatalf("nil Set Names() = %#v; want empty", got)
	}
}

func TestConcurrentAccess(t *testing.T) {
	set := New("always-on")
	var workers sync.WaitGroup

	for worker := 0; worker < 16; worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			for iteration := 0; iteration < 1_000; iteration++ {
				set.Set("changing", (worker+iteration)%2 == 0)
				_ = set.Enabled("changing")
				_ = set.Names()
			}
		}(worker)
	}

	workers.Wait()
	if !set.Enabled("always-on") {
		t.Fatal("concurrent updates changed an unrelated flag")
	}
}
