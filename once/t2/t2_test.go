/**
 *@Author:wangchao.zhuanxu
 *@Created At:2020/7/3 8:50 下午
 *@Description:
 */
package t2

import (
	"fmt"
	"sync"
	"testing"
	"unsafe"
)

type Singleton struct {
	data string
}
var (
	singleInstance *Singleton
	once sync.Once
)

func GetSingletonObj() *Singleton {
	once.Do(func() {
		singleInstance = new(Singleton)
	})
	return singleInstance
}

func TestGetSingletonObj(t *testing.T)  {
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			obj := GetSingletonObj()
			fmt.Printf("%X\n", unsafe.Pointer(obj))
			wg.Done()
		}()
	}
	wg.Wait()
}
