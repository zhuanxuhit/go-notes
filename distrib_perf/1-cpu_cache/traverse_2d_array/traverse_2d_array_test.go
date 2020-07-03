package traverse_2d_array

import (
	"testing"
)

const TESTN = 2048

func BenchmarkFastArray(t *testing.B) {
	matrix := make([][]int, TESTN)
	for i := 0; i < TESTN; i++ {
		matrix[i] = make([]int, TESTN)
	}
	for i := 0; i < t.N; i++ {
		FastArray(matrix, TESTN)
	}
}


func BenchmarkSlowArray(t *testing.B) {
	matrix := make([][]int, TESTN)
	for i := 0; i < TESTN; i++ {
		matrix[i] = make([]int, TESTN)
	}
	for i := 0; i < t.N; i++ {
		SlowArray(matrix, TESTN)
	}
}