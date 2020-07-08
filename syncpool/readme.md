t1是syncpool的使用示例

t1中是对bytepool的示例



## 知识点
https://time.geekbang.org/column/article/42160

问题 1：临时对象池存储值所用的数据结构是怎样的？

对象池中有本地池列表，不过更确切地说，它是一个数组。这个列表的长度，总是与 Go 语言调度器中的 P 的数量相同。

![](https://static001.geekbang.org/resource/image/82/22/825cae64e0a879faba34c0a157b7ca22.png)

问题 2：临时对象池是怎样利用内部数据结构来存取值的？
![](https://static001.geekbang.org/resource/image/df/21/df956fe29f35b41a14f941a9efd80d21.png)


