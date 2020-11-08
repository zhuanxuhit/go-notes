package concurrentmap

import "testing"
import "github.com/orcaman/concurrent-map"

type Animal struct {
	name string
}

func TestHas(t *testing.T) {
	m := cmap.New()

	// Get a missing element.
	if m.Has("Money") == true {
		t.Error("element shouldn't exists")
	}

	elephant := Animal{"elephant"}
	m.Set("elephant", elephant)

	if m.Has("elephant") == false {
		t.Error("element exists, expecting Has to return True.")
	}
}
