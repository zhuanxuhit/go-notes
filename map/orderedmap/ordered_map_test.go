package orderedmap

import (
	"github.com/stretchr/testify/assert"
	"testing"
)
import "github.com/elliotchance/orderedmap"

func TestOrderedMap_Front(t *testing.T) {
	t.Run("NilOnEmptyMap", func(t *testing.T) {
		m := orderedmap.NewOrderedMap()
		assert.Nil(t, m.Front())
	})

	t.Run("NilOnEmptyMap", func(t *testing.T) {
		m := orderedmap.NewOrderedMap()
		m.Set(1, true)
		assert.NotNil(t, m.Front())
	})
}
