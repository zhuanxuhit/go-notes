## 常见设计模式
http://tmrts.com/go-patterns/

https://blog.csdn.net/joy0921/article/details/80125194

### Concurrency Patterns
1. Parallelism

  相关文档：

  https://blog.golang.org/pipelines#TOC_8.

  https://www.oschina.net/translate/go-concurrency-patterns-pipelines?lang=chs&p=2

  主要是描述了管道的构建指南，下面是两个主要原则

  - 状态会在所有发送操作做完后，关闭它们的流出 channel
  - 状态会持续接收从流入 channel 输入的数值，直到 channel 关闭或者其发送者被释放。

  管道要么保证足够能存下所有发送数据的缓冲区，要么接收来自接收者明确的要放弃 channel 的信号，来保证释放发送者。

```go
func sq(done <-chan struct{}, in <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        defer close(out) // 发送完数据后最后记得关闭channel
        for n := range in { // 作为通道的写入方，要保证在写完后关闭的
            select {
            case out <- n * n: // 此处没有保证管道能存下所有数据，因此会导致外部如果不读取的话，此处会一直阻塞
            case <-done: // 要么接收来自接收者明确的要放弃 channel 的信号，来保证释放发送者
                return
            }
        }
    }()
    return out
}
```

例子：生成树摘要

2. 生成器：一次性生成一系列值；
3. 广播（Broadcast）：把一个消息同时传输到所有接收端；

### messaging消息传递模式

1. 扇入：该模块直接调用上级模块的个数，像漏斗型一样去工作
2. 扇出：该模块直接调用的下级模块的个数；