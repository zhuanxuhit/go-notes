/**
 *@Author:wangchao.zhuanxu
 *@Created At:2020/7/7 6:36 下午
 *@Description:
 */
package generators

import (
	"fmt"
	"testing"
)

func Count(start, end int) <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		for i := start; i <= end; i++ {
			ch <- i
		}
	}()
	return ch
}

func TestCount(t *testing.T) {
	fmt.Println("No bottles of beer on the wall")

	for i := range Count(1, 99) {
		fmt.Println("Pass it around, put one up,", i, "bottles of beer on the wall")
		// Pass it around, put one up, 1 bottles of beer on the wall
		// Pass it around, put one up, 2 bottles of beer on the wall
		// ...
		// Pass it around, put one up, 99 bottles of beer on the wall
	}

	fmt.Println(100, "bottles of beer on the wall")
}