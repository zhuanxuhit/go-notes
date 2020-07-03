package traverse_2d_array

func FastArray(arr [][]int, len int) {
	for i := 0; i < len; i++ {
		for j := 0; j < len; j++ {
			arr[i][j] = 0
		}
	}
}

func SlowArray(arr [][]int, len int) {
	for i := 0; i < len; i++ {
		for j := 0; j < len; j++ {
			arr[j][i] = 0
		}
	}
}
