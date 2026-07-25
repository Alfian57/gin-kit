package password

import "testing"

func TestHashCompareAndRehash(t *testing.T) {
	encoded, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !Compare("correct horse battery staple", encoded) || Compare("wrong", encoded) {
		t.Fatal("password comparison failed")
	}
	if New(DefaultParameters).NeedsRehash(encoded) {
		t.Fatal("default parameters unexpectedly need rehash")
	}
	changed := DefaultParameters
	changed.Iterations++
	if !New(changed).NeedsRehash(encoded) {
		t.Fatal("changed parameters did not require rehash")
	}
}
