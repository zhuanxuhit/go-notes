/**
 *@Author:wangchao.zhuanxu
 *@Created At:2020/7/4 8:29 上午
 *@Description:
 */
package t2

import (
	"fmt"
	"io"
	"testing"
)

func TestGetBuffer(t *testing.T) {
	buf := GetBuffer()
	defer buf.Free()
	buf.Write("A Pool is a set of temporary objects that" +
		"may be individually saved and retrieved.")
	buf.Write("A Pool is safe for use by multiple goroutines simultaneously.")
	buf.Write("A Pool must not be copied after first use.")

	fmt.Println("The data blocks in buffer:")
	for {
		block, err := buf.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(fmt.Errorf("unexpected error: %s", err))
		}
		fmt.Print(block)
	}
}
