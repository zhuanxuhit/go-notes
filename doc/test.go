package main

import (
	"fmt"
)

func main() {
	matrix := [][]int{
		{1, 2, 3, 4},
		{5, 6, 7, 8},
		{9, 10, 11, 12},
		{13, 14, 14, 16},
	}
	// m,n := 4,4
	// (0,0),(0,1),(0,2),(0,3)
	// (1,3,),(2,3), (3,3),
	// (2,3),(1,3),(0,3)
	// (0,2),(0,1)
	// (1,1), (1,2),
	// (2,2)
	// (2,1)
	// (1,1)
	data := make([]int, 0, 16)
	row_begin, row_end := 0, 4
	column_begin, column_end := 0, 4
	is_row := 1
	is_acent := 1 // 是否升序
	for i, j := row_begin, column_begin; ; {
		if is_row == 1 && is_acent == 1 {
			// 从左往右
			for i := row_begin; i < row_end; i++ {
				data = append(data, matrix[i][j])
			}
			is_row = 0
			is_acent = 1
			column_begin += 1
		} else if is_row == 0 && is_acent == 1 {
			// 从上往下
			for j = column_begin; j < column_end; j++ {
				data = append(data, matrix[i][j])
			}
			is_row = 1
			is_acent = 0
			row_end -= 1
		} else if is_row == 1 && is_acent == 0 {
			// 从右往左
			for j = row_end - 1; j >= row_begin; j-- {
				data = append(data, matrix[i][j])
			}
			is_row = 0
			is_acent = 0
			column_end -= 1
		} else {
			// 从下往上
			for j = column_end - 1; j >= column_begin; j-- {
				data = append(data, matrix[i][j])
			}
			is_row = 1
			is_acent = 1
			row_begin += 1
		}
		if len(data) == 16 {
			break
		}
	}
	fmt.Println(len(data), data)
}
