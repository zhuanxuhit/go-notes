/**
 *@Author:wangchao.zhuanxu
 *@Created At:2020/6/22 1:22 下午
 *@Description:
 */
package main

import (
	"fmt"
	"sync"
)

func main() {
	var nums sync.Map
	nums.Store(int64(1), 1)
	nums.Store(int64(2), 2)
	fmt.Println(nums)
	nums.Range(func(key, value interface{}) bool {
		num := key.(int64)
		val := value.(int)
		nums.Store(num*3, val*3)
		fmt.Println(num, val)
		return true
	})
	fmt.Println(nums)
}
