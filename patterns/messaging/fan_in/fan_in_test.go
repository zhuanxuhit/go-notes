/**
 *@Author:wangchao.zhuanxu
 *@Created At:2020/7/7 8:51 下午
 *@Description:
 */
package fan_in

import (
	"fmt"
	"sync"
	"testing"
)

// Merge different channels in one channel
func Merge(cs ...<-chan int) <-chan int {
	var wg sync.WaitGroup

	out := make(chan int)

	// Start an send goroutine for each input channel in cs. send
	// copies values from c to out until c is closed, then calls wg.Done.
	send := func(c <-chan int) {
		for n := range c {
			out <- n
		}
		wg.Done()
	}

	wg.Add(len(cs))
	for _, c := range cs {
		go send(c)
	}

	// Start a goroutine to close out once all the send goroutines are
	// done.  This must start after the wg.Add call.
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func gen(start, end int) <-chan int {
	c := make(chan int, end-start+1)
	for i := start; i <= end; i++ {
		c <- i
	}
	close(c)
	return c
}

func TestFanIn(t *testing.T) {
	c1 := gen(1, 10)
	c2 := gen(20, 23)
	c3 := Merge(c1, c2)
	for i := range c3 {
		fmt.Printf("read %d\n", i)
	}
}
