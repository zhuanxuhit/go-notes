从C10K问题谈起



C10K问题描述：如何在一台物理机上同时服务 10000 个用户？这里 C 表示并发，10K 等于 10000。



## bthread基础

需要了解的一些基础知识，以下文章来自[Coroutine in Depth - Context Switch](http://jinke.me/2018-09-14-coroutine-context-switch/)

### Stack Frame

具体分为 Caller Stack Frame 和 Callee Stack Frame，其关系如下：

```c++
int add(int x, int y) {
    int res = x + y;
    return res;
}
 
int main() {
    add(1, 2);
    return 0;
}		
```

Caller Stack Frame 如下：

![Caller Stack Frame](https://raw.githubusercontent.com/zhuanxuhit/pic_store/master/20200810203400.png)

调用add函数的时候，会将参数从右到左反向压入栈，同时将返回地址压栈，相应汇编代码如下：

```asm
push 2
push 1
 
// 汇编为 call _add，翻译如下：
push eip + 2  // eip + 2 即为 Return Address
jmp _add
```

进入到add函数中，编译器为我们生成的 Callee Stack Frame 如下：

![img](https://raw.githubusercontent.com/zhuanxuhit/pic_store/master/20181222103325.png)

汇编代码为：

```assembly
push ebp          // 将当前 ebp 寄存器的值（Caller Stack Frame 的栈基指针）push 到栈中
move ebp esp      // 将当前 esp 寄存器的值（即 Callee Stack Frame 的栈基指针）放到 ebp 寄存器中保存
sub esp X;        // 将 esp 寄存器（栈顶指针）往下移 X，为本地变量预留出栈上空间
```

这段代码的主要目的为：

1. 将旧的 ebp push 到栈中（函数调用结束后会恢复该值）
2. 将当前 Stack Frame 的栈基指针赋值给 ebp 寄存器（这样做的目的是编译器可以通过 ebp 寄存器的值定位到 Caller 参数和 Callee Local Variable
3. 为本地变量分配空间。

当函数执行完后，就是函数返回逻辑了，编译器生成的汇编代码如下：

```assembly
mov esp ebp     // esp=ebp, 将 esp （栈顶指针）赋值为 ebp 寄存器（即之前保持的栈基指针）
pop ebp         // 将当前栈顶值（即 old ebp）pop 给 ebp 寄存器（这样子就在函数调用后恢复了 ebp 寄存器）
ret             // 将当前栈顶值（即 Return Address）pop 给 eip 寄存器（指令寄存器）
// ret 可理解为以下汇编代码
add esp, 8      // 将 esp（栈顶指针）上移 8（Caller 为函数参数分配的空间大小）
pop eip
```

有了stackframe基础后，我们下面来看bthread是如何进行切换的。

### context Switch

bthread 中与上下文切换相关的操作分为两个步骤：

```c++
// 跳转 bthread context
intptr_t BTHREAD_CONTEXT_CALL_CONVENTION
bthread_jump_fcontext(bthread_fcontext_t * ofc, bthread_fcontext_t nfc,
                      intptr_t vp, bool preserve_fpu = false);
// 创建 bthread context
bthread_fcontext_t BTHREAD_CONTEXT_CALL_CONVENTION
bthread_make_fcontext(void* sp, size_t size, void (* fn)( intptr_t));
```

在看这两个函数的具体汇编实现之前，先列出后续用到的几个寄存器信息（以 x86_64 代码为例，好读点）：

| 寄存器（64位） | 含义                     |
| :------------- | :----------------------- |
| rdi            | 保存 Caller 第一个参数   |
| rsi            | 保存 Caller 第二个参数   |
| rdx            | 保存 Caller 第三个参数   |
| rcx            | 保存 Caller 第四个参数   |
| rax            | 保存 Callee 第一个返回值 |
| rsp            | 堆栈顶指针               |
| rbp            | 堆栈基指针               |
| rip            | 指令寄存器               |

我们先来看 bthread context 的创建：

```assembly
__asm (
".text\n"
".globl bthread_make_fcontext\n"
".type bthread_make_fcontext,@function\n"
".align 16\n"
"bthread_make_fcontext:\n"
"    movq  %rdi, %rax\n"            // 将 rdi 的值（即第一个参数 sp，也就是协程栈空间的地址）赋值给 rax，此时 rax 指向用户自定义协程栈的地址
"    andq  $-16, %rax\n"            // 将 rax 下移 16 字节对齐
"    leaq  -0x48(%rax), %rax\n"     // 将 rax 下移 0x48，即留出 0x48 个字节用于存放 Context Data
"    movq  %rdx, 0x38(%rax)\n"      // 将 rdx 的值（即第三个参数 fn，也就是协程初始化后的运行函数）赋值给 (rax) + 0x38 的位置上
"    stmxcsr  (%rax)\n"             // 存储 MMX Control 信息
"    fnstcw   0x4(%rax)\n"          // 存储 x87 Control 信息
"    leaq  finish(%rip), %rcx\n"    // 计算 finish 对应的指令地址，存放在 rcx 中
"    movq  %rcx, 0x40(%rax)\n"      // 将 rcx 的值赋值给 (rax) + 0x40 的位置上
"    ret \n"
"finish:\n"                         // 函数退出
"    xorq  %rdi, %rdi\n"
"    call  _exit@PLT\n"
"    hlt\n"
".size bthread_make_fcontext,.-bthread_make_fcontext\n"
".section .note.GNU-stack,\"\",%progbits\n"
);
```

`bthread_make_fcontext` 的汇编还算容易懂，基于此，我们可以画出 Coroutine Stack 初始化后的 Layout：

![img](https://raw.githubusercontent.com/zhuanxuhit/pic_store/master/20181222103349.png)

其中 rax 寄存器就是 `bthread_make_fcontext` 的返回值，也就是 `bthread_fcontext_t`。

终于到 Context Switch 的环节了，我们来看 `bthread_jump_fcontext` 的代码：

```assembly
__asm (
".text\n"
".globl bthread_jump_fcontext\n"
".type bthread_jump_fcontext,@function\n"
".align 16\n"
"bthread_jump_fcontext:\n"
"    pushq  %rbp  \n"                     // push rbp 寄存器值到当前协程栈中
"    pushq  %rbx  \n"                     // push rbx 寄存器值到当前协程栈中
"    pushq  %r15  \n"                     // push r15 寄存器值到当前协程栈中
"    pushq  %r14  \n"                     // push r14 寄存器值到当前协程栈中
"    pushq  %r13  \n"                     // push r13 寄存器值到当前协程栈中
"    pushq  %r12  \n"                     // push r12 寄存器值到当前协程栈中
"    leaq  -0x8(%rsp), %rsp\n"            // rsp （栈顶指针）下移 8 字节
"    cmp  $0, %rcx\n"                     // 比较 rcx（第四个入参，即 preserve_fpu，默认为 false）与 0
"    je  1f\n"                            // rcx = 0（即 preserve_fpu 为 false）则跳转
"    stmxcsr  (%rsp)\n"                   // 保存 MMX Control 信息
"    fnstcw   0x4(%rsp)\n"                // 保存 x87 Control 信息
"1:\n"
"    movq  %rsp, (%rdi)\n"                // 将 rsp（栈顶指针）保存在 rdi 寄存器所指向的值（即第一个入参，ofc）中
"    movq  %rsi, %rsp\n"                  // 将 rsp（栈顶指针）赋值为 rsi（即第二个入参，nfc）
"    cmp  $0, %rcx\n"                   
"    je  2f\n"
"    ldmxcsr  (%rsp)\n"
"    fldcw  0x4(%rsp)\n"
"2:\n"
"    leaq  0x8(%rsp), %rsp\n"             // rsp（栈顶）指针上移 8 字节
"    popq  %r12  \n"                      // pop 栈顶元素，赋值给 r12
"    popq  %r13  \n"                      // pop 栈顶元素，赋值给 r13
"    popq  %r14  \n"                      // pop 栈顶元素，赋值给 r14
"    popq  %r15  \n"                      // pop 栈顶元素，赋值给 r15
"    popq  %rbx  \n"                      // pop 栈顶元素，赋值给 rbx
"    popq  %rbp  \n"                      // pop 栈顶元素，赋值给 rbp
"    popq  %r8\n"                         // pop 栈顶元素，赋值给 r8
"    movq  %rdx, %rax\n"                  // 将 rdx 赋值给 rax，作为返回值
"    movq  %rdx, %rdi\n"                  // 将 rdx 赋值给 rdi，作为接下来的入参
"    jmp  *%r8\n"                         // 跳转到 r8 寄存器指示的位置处继续执行
".size bthread_jump_fcontext,.-bthread_jump_fcontext\n"
".section .note.GNU-stack,\"\",%progbits\n"
);
```

Context Switch 的过程就显得有些复杂了。当然，核心思想还是保持和恢复两个 Coroutine 的寄存器和栈信息。不过，这里需要针对被调度 Coroutine 分为两种情况讨论：首次调度、非首次调度。我分别画出相应的 Stack Frame Layout 加以说明：

首次调度是指该 Coroutine 的 Stack Frame 如 `make_fcontext` 中画出的那样，于是 Context Switch 意味着：

![img](https://raw.githubusercontent.com/zhuanxuhit/pic_store/master/20181222103410.png)

非首次调度是指该 Coroutine 已经运行过一段时间了，那么其 Stack Frame 就如上图右二所示，于是 Context Switch 意味着：

![img](https://sleepy-1256633542.cos.ap-beijing.myqcloud.com/20181222103426.png)

瞧！这里面最精妙的就是首次调度和非首次调度之间细微的差别。首次调度继续运行的地址（R8）就是 make context 时保存的 fn（协程入口函数，往往与调度器有关），非首次调度继续运行的地址（R8）则恰好就是我们在第 2 节（Stack Frame）一再强调的 Return Address，也就是协程被切换的位置。



## bthread调度

### 任务定义TaskMeta

brpc中一个任务被称为bthread，一个bthread任务在内存中表示为一个`TaskMeta`对象，`TaskMeta`对象为保证分配的效率，使用`ResourcePool`进行分配。

其中`TaskMeta`类的主要成员有：

```c++
struct TaskMeta {
	// 应用程序设置的函数和参数
  void* (*fn)(void*);
  void* arg;
  // 任务栈
  ContextualStack* stack;
  // 用于记录一些bthread运行状态（如各类统计值）等的一块内存
  LocalStorage local_storage;
  // 指向一个Butex对象头节点的指针。
  uint32_t* version_butex;
}

struct ContextualStack {
    // 任务私有栈
    bthread_fcontext_t context;
   // 任务栈类型
    StackType stacktype;
  // 任务栈
    StackStorage storage;
};
```

栈类型

| 栈类型             | 说明                                   | 大小          |
| :----------------- | :------------------------------------- | :------------ |
| STACK_TYPE_MAIN    | worker pthread的栈                     | 默认大小是8MB |
| STACK_TYPE_PTHREAD | 使用worker pthread的栈，不需要额外分配 | 默认大小是8MB |
| STACK_TYPE_SMALL   | 小型栈                                 | 32KB          |
| STACK_TYPE_NORMAL  | 默认栈                                 | 1MB           |
| STACK_TYPE_LARGE   | 大型栈                                 | 8MB           |

介绍完bthread的主要结构`TaskMeta`，下面我们来看下bthread是如何调度起来的。



brpc在N:1协程的基础上做了扩展，实现了M:N用户级线程，即N个pthread系统线程去调度执行M个协程（M远远大于N），一个pthread有其私有的任务队列，队列中存储等待执行的若干协程，一个pthread执行完任务队列中的所有协程后，也可以去其他pthread的任务队列中拿协程任务，即work-steal机制，这样的话如果一个协程在执行较为耗时的操作时，同一任务队列中的其他协程有机会被调度到其他pthread上去执行，从而实现了全局的最大并发。并且brpc也实现了协程级的互斥与唤醒，即Butex机制，通过Butex，一个协程在等待网络I/O或等待互斥锁的时候，会被自动yield让出cpu，在适当时候会被其他协程唤醒，恢复执行。关于Butex的详情参见[这篇文章](https://github.com/ronaldo8210/brpc_source_code_analysis/blob/master/docs/butex.md)。



### pthread抽象TaskGroup

每一个TaskGroup对象是系统线程pthread的线程私有对象，它内部包含有任务队列，并控制pthread如何执行任务队列中的众多bthread任务，通过__thread确保每个pthread只会有一个TaskGroup对象。

```c++
// task_group.cpp 中定义
__thread TaskGroup* tls_task_group = NULL;
```

TaskGroup对象的主要成员有：

- `RemoteTaskQueue _remote_rq`，如果一个pthread 1想让pthread 2执行bthread 1，则pthread 1会将bthread 1的tid压入pthread 2的TaskGroup的_remote_rq队列中。
  - 具体执行点：
- `WorkStealingQueue<bthread_t> _rq`：pthread 1在执行从自己私有的TaskGroup中取出的bthread 1时，如果bthread 1执行过程中又创建了新的bthread 2，则bthread 1将bthread 2的tid压入pthread 1的TaskGroup的_rq队列中。
  - 具体的执行点：
- `_main_tid & _main_stack`：一个pthread会在TaskGroup::run_main_task()中执行while()循环，不断获取并执行bthread任务，一个pthread的执行流不是永远在bthread中，比如等待任务时，pthread没有执行任何bthread，执行流就是直接在pthread上。可以将pthread在“等待bthread-获取到bthread-进入bthread执行任务函数之前”这个过程也抽象成一个bthread，称作一个pthread的“调度bthread”或者“主bthread”，它的tid和私有栈就是`_main_tid`和`_main_stack`。
- `_cur_meta`：当前正在执行的bthread的TaskMeta对象的地址。

### pthread管理者TaskControl

TaskControl对象是全局单例的，获取方法是通过`get_or_new_task_control`函数，因为是全局单例，所以我们得保证只初始化一次的，如何保证的呢？通过atomic保证原子性读写，具体实现如下：

```c++
inline TaskControl* get_or_new_task_control() {
    // double check
    auto p = (butil::atomic<TaskControl*>*)&g_task_control;
  	// memory_order_consume 保证其他线程的改变此处肯定能看到
    TaskControl* c = p->load(butil::memory_order_consume);
    if (c != NULL) {
        return c;
    }
  	// double check
    BAIDU_SCOPED_LOCK(g_task_control_mutex);
    c = p->load(butil::memory_order_consume);
    if (c != NULL) {
        return c;
    }
    c = new (std::nothrow) TaskControl;
    if (NULL == c) {
        return NULL;
    }
    int concurrency = FLAGS_bthread_min_concurrency > 0 ?
        FLAGS_bthread_min_concurrency :
        FLAGS_bthread_concurrency;
    // task control 初始化函数
    if (c->init(concurrency) != 0) {
        LOG(ERROR) << "Fail to init g_task_control";
        delete c;
        return NULL;
    }
    p->store(c, butil::memory_order_release);
    return c;
}
```

此处有个点：为什么不将`g_task_control`声明为`atomic<TaskControl*>`？有个[issue做了回答](https://github.com/apache/incubator-brpc/issues/246)，总结来说就是：

> TaskControl* g_task_control = NULL; 在 static initialization（使用zero initialization语义）阶段完成，而atomic<TaskControl*> g_task_control; 在 dynamic initialization阶段完成，所以存在race。

# 问题

**怎么理解上面这个原因呢？？？？？**



另外一个为啥此处用`memory_order_consume`和`memory_order_release`的问题，有个[issue](https://github.com/apache/incubator-brpc/issues/168)也做了回答：

> 此处p->store(c, butil::memory_order_release)和p->load(butil::memory_order_consume)是配对的，保证线程对p的更改能够被后续所有线程看到。



当我们第一次新建出TaskControl对象后，我们需要进行初始化，TaskControl中主要做了下面几件事情：

1. 创建TimerThread
2. 创建`_concurrency`个pthread

其中每个pthread，都执行`worker_thread`函数，这个函数主要做：

- 创建一个 `TaskGroup` 对象，调用 `init` 函数初始化完成后加入到 `TaskControl` 中。
- 然后调用 `run_main_task` 函数，开始执行main_thread循环：等待bthread-获取到bthread-进入bthread执行任务函数。

worker pthread在任意时刻只会运行一个bthread。它优先运行远程队列的bthread，如果没有，就从其它TaskGroup的本地队列或远程队列中偷取。如果仍然没有找到，就会睡眠直到有新的bthread可以运行时被唤醒。



## bthread生命周期

下面来看一个bthread的生命周期，

### 创建

主要有两个函数：

- bthread_start_urgent

- bthread_start_background

### 执行

bthread执行过程中，又可以细分为下面3个状态：

1. bthread的任务处理函数执行完成。一个bthread的任务函数结束后，该bthread需要负责查看TaskGroup的任务队列中是否还有bthread，如果有，则pthread执行流直接进入下一个bthread的任务函数中去执行；如果没有，则执行流返回pthread的调度bthread，等待其他pthread传递新的bthread；
   - bthread首次跳转过来执行的是`task_runner`函数
   - 当执行完后，会调用`ending_sched`查看队列中是否还有其他待运行的bthread，有的话调用`sched_to`直接切换过去
2. bthread在任务函数执行过程中yield挂起，则pthread去执行任务队列中下一个bthread，如果任务队列为空，则执行流返回pthread的调度bthread，等待其他pthread传递新的bthread。挂起的bthread何时恢复运行取决于具体的业务场景，它应该被某个bthread唤醒，与pthread的调度无关。这样的例子有负责向TCP连接写数据的bthread因等待inode输出缓冲可写而被yield挂起、等待Butex互斥锁的bthread被yield挂起等。
3. bthread在任务函数执行过程中可以创建新的bthread，因为新的bthread一般是优先级更高的bthread，所以pthread执行流立即进入新bthread的任务函数，原先的bthread被重新加入到任务队列的尾部，不久后它仍然可以被pthread执行。但由于work-steal机制，它不一定会在原先的pthread执行，可能会被steal到其他pthread上执行。



## main_thread

初始化时机：`TaskGroup::init`，main thread不用设置自己的堆栈，直接使用pthread的即可。

主函数：`run_main_task`，







Native POSIX Thread Library (NPTL) 

[设计文档](https://www.akkadia.org/drepper/nptl-design.pdf)

[Measuring context switching and memory overheads for Linux threads](https://eli.thegreenplace.net/2018/measuring-context-switching-and-memory-overheads-for-linux-threads/)

[Launching Linux threads and processes with clone](https://eli.thegreenplace.net/2018/launching-linux-threads-and-processes-with-clone/)























## 备份

[bthread](https://github.com/brpc/brpc/tree/master/src/bthread)是brpc使用的M:N线程库，目的是在提高程序的并发度的同时，降低编码难度，并在核数日益增多的CPU上提供更好的scalability和cache locality。



- M:N线程库
- 提高并发度同时，降低编码难度
- 如何在核数日益增多的CPU上提供更好的scalability和cache locality



什么是M:N的线程库？

- bthread_start_urgent

## 关键数据结构

### bthread

bthread是实现**M:N线程库**的关键，一个pthread，我们希望能够同时运行多个bthread，那我们就得在用户态去保存bthread的执行信息，这个主要包括两部分：

- 栈信息
- 寄存器信息

这些是能够让bthread恢复执行的关键，所以抽象出`TaskMeta`结构体来对bthread相关信息进行记录。





```c++
typedef unsigned bthread_stacktype_t; // 类型有：
typedef unsigned bthread_attrflags_t; // flag有：
typedef struct {
    pthread_mutex_t mutex;
    void* free_keytables;
    int destroyed;
} bthread_keytable_pool_t;

typedef struct bthread_attr_t {
    bthread_stacktype_t stack_type;
    bthread_attrflags_t flags;
    bthread_keytable_pool_t* keytable_pool;
}
```

栈类型

| 栈类型             | 说明                                   | 大小          |
| :----------------- | :------------------------------------- | :------------ |
| STACK_TYPE_MAIN    | worker pthread的栈                     | 默认大小是8MB |
| STACK_TYPE_PTHREAD | 使用worker pthread的栈，不需要额外分配 | 默认大小是8MB |
| STACK_TYPE_SMALL   | 小型栈                                 | 32KB          |
| STACK_TYPE_NORMAL  | 默认栈                                 | 1MB           |
| STACK_TYPE_LARGE   | 大型栈                                 | 8MB           |

栈分配是通过工厂函数：



#### 马上启动任务:bthread_start_urgent

```c++
int bthread_start_urgent(bthread_t* __restrict tid,
                         const bthread_attr_t* __restrict attr,
                         void * (*fn)(void*),
                         void* __restrict arg) {
    // tls_task_group 什么时候初始化呢？
    // TaskControl::worker_thread 中会设置 tls_task_group 的值
    // worker_thread 函数是会在 TaskControl::init 中创建 pthread 时设置
    // worker_thread 是工作线程的主要函数
    // 所有当g存在，说明我们是在工作线程中启动的
    bthread::TaskGroup* g = bthread::tls_task_group;
    if (g) {
        // start from worker
        return bthread::TaskGroup::start_foreground(&g, tid, attr, fn, arg);
    }
    return bthread::start_from_non_worker(tid, attr, fn, arg);
}
```

先来看`start_from_non_worker`，即从非工作线程启动的流程：

- 如果是第一次获取task_control，会做响应的初始化，建立worker thread
- 如何设置了bthread属性：BTHREAD_NOSIGNAL，则通过tls_task_group_nosignal来启动bthread
  - 为什么要记录启动BTHREAD_NOSIGNAL的TaskGroup？
    - NOSIGNAL一般是批量创建bthreads场景，将他们放入同一个TaskGroup能够最大化batch
    - bthread_flush()需要知道哪个TaskGroup进行flush操作
- 否则通过TaskControl任意选择一个（choose_one_group()）TaskGroup来启动（start_background）bthread
  - 启动bthread，分为是本worker thread启动还是不是，然后分别调用下面两个函数：
    - `ready_to_run_remote`
      - 如果发现remote队列已经满了，尝试做下面的事情【目的是让睡眠的workerThread去执行任务】
        1. 清空_remote_num_nosignal（如果不为0）
        2. 将_remote_num_nosignal累加到\_remote_nsignaled
        3. TaskControl调用signal_task，触发_remote_num_nosignal个任务
           1. 唤醒睡眠的workerThread进行处理
           2. 如果没有可唤醒的，则会通过`add_workers`新增workerThread
    - `ready_to_run`
      - 如果发现本地队列已满，此时我们没有将任务放入到其他TaskGroup中，因为
        1. 本地已经满，很可能其他TaskGroup也已经有很多任务了，将任务插入到其他TaskGroup帮助不大
        2. 实验发现，采用插入到其他TaskGroup的方式在所有TaskGroup都在创建task的时候，性能不好
      - 

再看来`start_foreground`函数





### TaskMeta

```c++
struct TaskMeta {
	// 最近本的函数和参数
  void* (*fn)(void*);
  void* arg;
  // 任务栈
  ContextualStack* stack;
  // 这个的作用是？
  LocalStorage local_storage;
}
```

Task的主函数是：TaskGroup::task_runner，是在stack切换时的入口函数。



### TaskGroup

问：什么时候创建TaskGroup的？

答：TaskControl::worker_thread 中会设置 tls_task_group 的值，具体来说是通过TaskControl的`create_group()`方法来创建的。



```c++
// thread local
class TaskGroup {

}
```



#### TaskControl::create_group

这个函数是在TaskContro中的，创建`TaskGroup`流程：

```c++
TaskGroup* TaskControl::create_group() {
  TaskGroup* g = new (std::nothrow) TaskGroup(this);
	g->init(FLAGS_task_group_runqueue_capacity); // 4096 大小
  _add_group(g);
}
```

上面初始化`init`中做的事情是：

- 初始化 `_rq` 队列，用来存储worker pthread创建的bthread。队列中的bthread可能被其他TaskGroup偷取。
- 初始化 `_remote_rq` 队列，大小为 `_rq` 队列的一半，用来存储non-worker pthread创建的bthread。队列中的bthread可能被其他TaskGroup偷取。
- 为worker pthread分配 `TaskMeta` 对象和栈。

##### _add_group

将新创建的TaskGroup放入到TaskControl的\_groups数组中，并且更新\_ngroup。



### TaskControl

TaskControl用于管理brpc创建的worker pthread。

```c++
// 管理所有的taskGroup
class TaskControl {
private:
  WorkStealingQueue<bthread_t> _rq; // run queue是设么？
  RemoteTaskQueue _remote_rq;
}
```

什么时候创建TaskControl呢？在函数`get_or_new_task_control`中。

有一个全局变量：`TaskControl* g_task_control`，为了控制并发访问，于是有一个锁：`pthread_mutex_t g_task_control_mutex`

#### func：get_or_new_task_control

```c++
inline TaskControl* get_or_new_task_control() { 
  // 主要是查看g_task_control是否设置了值，如果没有，则调用init函数
  // ....
  c = new (std::nothrow) TaskControl;
  c->init(concurrency);
  // ...
}

// 初始化函数
int TaskControl::init(int concurrency) {
	// ...
  // 
  get_or_create_global_timer_thread(); // 初始化timer control
  // 创建 _concurrency 个thread，worker_thread 是执行函数
  pthread_create(&_workers[i], NULL, worker_thread, this);
  // 等待至少一个worker pthread创建完成
  // ...  
}

// 获取timer_thread
TimerThread* get_or_create_global_timer_thread() {
    pthread_once(&g_timer_thread_once, init_global_timer_thread);
    return g_timer_thread;
}
// 初始化 timer_thread
static void init_global_timer_thread() {
	g_timer_thread = new (std::nothrow) TimerThread;
  TimerThreadOptions options;
  options.bvar_prefix = "bthread_timer";
  g_timer_thread->start(&options);
}
```

#### func: worker_thread

每个worker pthread运行 `worker_thread` 函数，这个函数主要做：

- 创建一个 `TaskGroup` 对象，调用 `init` 函数初始化完成后加入到 `TaskControl` 中。在brpc中，每个worker pthread有各自的TaskGroup。
- 然后调用 `run_main_task` 函数，开始调用bthread。

worker pthread在任意时刻只会运行一个bthread。它优先运行本地队列，然后是远程队列的bthread，如果没有，就从其它TaskGroup的本地队列或远程队列中偷取。如果仍然没有找到，就会睡眠直到有新的bthread可以运行时被唤醒。



此处有两个关键队列：

- 本地队列`WorkStealingQueue<bthread_t> _rq;`为什么叫StealingQueue，因为可能会被其他TaskGroup偷走
- 远程队列`RemoteTaskQueue _remote_rq;`

##### 主体函数：run_main_task

- wait_task
  - 等待`ParkingLot`被唤醒，`ParkingLot`是调度相关，可以看[下面](#bthread调度)
  - `steal_task`偷取任务
    - 如果 _remote_rq 中有任务，直接返回
    - 否则调用TaskControl的steal_task函数`_control->steal_task(tid, &_steal_seed, _steal_offset);`
      - 依次按规则从各个woker里取，优先本地队列，后remote队列。
        - `g->_rq.steal(tid)`
          - 此处本地队列结构是：`WorkStealingQueue`，可以看下面的数据结构[WorkStealingQueue](#WorkStealingQueue)
        - `g->_remote_rq.pop(tid)`
          - 此处remote队列结构是：`RemoteTaskQueue`
- sched_to
  - 检查是否有设置堆栈，设置成功后，调用内部的sched_to函数
    - 内部sched_to
      - 



### TimerThread

用来执行周期性任务，任务通过数据结构`Task`来表示

```c++
class TimerThread {

}
// 此方法只会调用一次，是通过 init_global_timer_thread 方法来保证的
int TimerThread::start(const TimerThreadOptions* options_in) {
	// 启动
 	// 创建线程
  pthread_create(&_thread, NULL, TimerThread::run_this, this);
}
// run_this
void TimerThread::run() {
	// 通过 bthread_set_worker_startfn 设置 thread 启动运行函数
}
```





#### Bucket

不同的task会放入不同的bucket，减少竞争。

**数据结构**

```c++
class BAIDU_CACHELINE_ALIGNMENT TimerThread::Bucket {
private:
  // 互斥锁
    internal::FastPthreadMutex _mutex;
  // 
    int64_t _nearest_run_time;
  // task链表头
    Task* _task_head;
}
```



- 核心方法
  - consume_tasks
    - 从_task_head中获取最近要执行的Task
  - schedule：将任务放入运行队列
    - 





#### Task

`task`是一个链表，

```c++
struct BAIDU_CACHELINE_ALIGNMENT TimerThread::Task {
	void (*fn)(void*);          // the fn(arg) to run
  void* arg; // 参数
  // 执行时间
  int64_t run_time;           // run the task at this realtime
}
```

#### 整体流程

![image-20200804140946788](/Users/zhuanxu/Library/Application Support/typora-user-images/image-20200804140946788.png)



### ResourcePool

对象池，帮助我们复用对象。



### WorkStealingQueue

一篇有用的[参考](https://blog.csdn.net/zhougb3/article/details/104739781)

内部是一个数组，容量是`_capacity`，通过\_bottom和\_top两个指针来标识数据区域。

此处注意下面的两点即可：

- 本地工作线程往队列尾部push和pop队列数据，push和pop不存在并发问题
- 其余竞争线程往队列首部steal队列数据，会和push和pop存在竞争问题



### RemoteTaskQueue

RemoteTaskQueue主要用来存储非workerthread上新建的bthread，其结构主要是：

```c++
butil::BoundedQueue<bthread_t> _tasks;
butil::Mutex _mutex;
```

通过mutex来保证线程安全，通过BoundedQueue来管理元素，BoundedQueue是一个有容量限制的数组，其元素在入队时，会在内部上构建元素，

```c++
bool push(const T& item) {
  if (_count < _cap) {
    // 直接在原地调用拷贝构造
    new ((T*)_items + _mod(_start + _count, _cap)) T(item);
    ++_count;
    return true;
  }
  return false;
}
```



### Butex

由于brpc中引入了bthread，如果在bthread中使用了mutex，那么将会挂起当前pthread，导致该bthread_worker无法执行其他bthread，因此类似pthread和futex的关系，brpc引入butex来实现bthread粒度的挂起和唤醒。

```c++
struct BAIDU_CACHELINE_ALIGNMENT Butex {
    Butex() {}
    ~Butex() {}

    butil::atomic<int> value;
  // 当bthread挂起的时候放入该队列
    ButexWaiterList waiters;
    internal::FastPthreadMutex waiter_lock;
};
```

相关api

- 创建butex：void* butex_create()
- 删除butex： butex_destroy(void* butex)



## 关键流程

### bthread 调度

关键数据结构`ParkingLot`类只有一个成员变量：

```c++
// higher 31 bits for signalling, LSB for stopping.
butil::atomic<int> _pending_signal;
```

在`TaskControl`中定义了`PARKING_LOT_NUM`个`ParkingLot`，

```
static const int PARKING_LOT_NUM = 4;
ParkingLot _pl[PARKING_LOT_NUM];
```

在每个Task Group在构造函数中，会初始化变量：

```c++
_pl = &c->_pl[butil::fmix64(pthread_numeric_id()) % TaskControl::PARKING_LOT_NUM];
```

在每个worker_thread函数中，都会通过`ParkingLot`进行wait，等待被唤醒。



### TaskControl.signal_task

这个函数挺有意思，用于唤醒没有任务在处理的workerThread，

```c++
void TaskControl::signal_task(int num_task) {
    if (num_task <= 0) {
        return;
    }
	  // 为什么if (num_task > 2) num_task = 2;这样多余的任务不是没有被取走
    if (num_task > 2) {
        num_task = 2;
    }
    int start_index = butil::fmix64(pthread_numeric_id()) % PARKING_LOT_NUM;
    // 优先唤醒本workerThread，workerThread都wait在 run_main_task函数的 wait_task上
    num_task -= _pl[start_index].signal(1);
    if (num_task > 0) {
        //
        for (int i = 1; i < PARKING_LOT_NUM && num_task > 0; ++i) {
            if (++start_index >= PARKING_LOT_NUM) {
                start_index = 0;
            }
            num_task -= _pl[start_index].signal(1);
        }
    }
    if (num_task > 0 &&
        FLAGS_bthread_min_concurrency > 0 &&    // test min_concurrency for performance
        _concurrency.load(butil::memory_order_relaxed) < FLAGS_bthread_concurrency) {
        // TODO: Reduce this lock
        BAIDU_SCOPED_LOCK(g_task_control_mutex);
        if (_concurrency.load(butil::memory_order_acquire) < FLAGS_bthread_concurrency) {
            add_workers(1);
        }
    }
}
```





## Butex

为什么会需要Butex，因为mutex是提供给线程的，使用后会让线程睡眠，而bthread只是让出当前bthread，不会引起pthread的睡眠，下面先看`butex`定义：

```c++
struct BAIDU_CACHELINE_ALIGNMENT Butex {
    Butex() {}
    ~Butex() {}

    butil::atomic<int> value;
    ButexWaiterList waiters;
    internal::FastPthreadMutex waiter_lock;
};
```

先来看





## 参数

- bthread_concurrency、bthread_min_concurrency

  - 最开始启动最少的thread：bthread_min_concurrency，然后后续按需启动到bthread_concurrency

  



## 知识点

### __builtin_expect

能更好的利用pipeline，详细看[回答](https://stackoverflow.com/questions/7346929/what-is-the-advantage-of-gccs-builtin-expect-in-if-else-statements)

### __restrict

能让编译器更好的优化，详细[回答](https://www.zhihu.com/question/41653775)

### std::atomic

最重要的是几个mem_order的使用





### new (std::nothrow)

在内存耗尽的时候，new不抛出`std::bad_alloc`，而是返回null



### 类型转换运算符

- `static_cast<dest_type>(src_obj)`
  - 作用相当于C风格的强制转换，但是在多重继承的情况下，它会正确地调整指针的值，而C风格的强制转换则不会调整；它可以遍历继承树来确定src_obj与dest_type的关系，但是只在编译时进行（此所谓静态）；如果使用它来做downcast操作，则会存在隐患。

- `const_cast<dest_type>(src_obj)`
  - 用于去除一个对象的const/volatile属性
- `reinterpret_cast<dest_type>(src_obj)`
  - 我们可以借助它把一个整数转换成一个地址，或者在任何两种类型的指针之间转换。使用该运算符的结果很危险，请你不要轻易使用
- `dynamic_cast<dest_type>(src_obj)`
  - 在运行时遍历继承树（类层次结构）来确定src_obj与dest_type的关系

![类型转换](https://raw.githubusercontent.com/zhuanxuhit/pic_store/master/image-20200803210019165.png)

###  

>  \#include <linux/futex.h>
>  \#include <sys/time.h>
>   int futex (int *uaddr, int op, int val, const struct timespec *timeout,int *uaddr2, int val3);
>   \#define __NR_futex       240
>   虽然参数有点长，其实常用的就是前面三个，后面的timeout大家都能理解，其他的也常被ignore。
>   uaddr就是用户态下共享内存的地址，里面存放的是一个对齐的整型计数器。
>   op存放着操作类型。定义的有5中，这里我简单的介绍一下两种，剩下的感兴趣的自己去man futex
>   FUTEX_WAIT: 原子性的检查uaddr中计数器的值是否为val,如果是则让进程休眠，直到FUTEX_WAKE或者超时(time-out)。也就是把进程挂到uaddr相对应的等待队列上去。
>   FUTEX_WAKE: 最多唤醒val个等待在uaddr上进程。







## 杂

### 什么是 _remote_num_nosignal 任务？

就是这些任务是nosignal方式加入的任务，什么是nosignal呢？就是这些任务不需要马上执行的，可以积攒，然后被一起调度执行。

### 什么叫 current_pthread_task？

taskGroup中`_cur_meta->stack == _main_stack`，如果当前的Task执行模式是pthread-mode。







![x86-64寄存器地址](https://raw.githubusercontent.com/zhuanxuhit/pic_store/master/v2-79260f399bfc694bc0bccab477da3b90_1440w.jpg)

如上为寄存器的基本信息

1. %rax: 通常用于存储函数调用的返回结果
2. %rsp：栈指针寄存器，通常指向栈顶为止，push/pop操作就是通过改变 %rsp的值移动栈指针进行实现
3. %rbp：栈帧指针，用于表示当前栈帧的起始位置 （用于获取父函数压栈的参数）
4. %rdi, %rsi, %rdx, %rcx,%r8, %r9：六个寄存器用于依次存储函数调用时的6个参数(超过6个的参数会压堆栈)
5. miscellaneous registers: 此类寄存器更加通用广泛的寄存器，编译器或汇编程序可以根据需要存储任何数据
6. Caller Save & Callee Save： 即表示寄存器是由“调用者保存”还是由“被调用者保存”，产生函数调用的时候，通用寄存器也会被子函数调用，为了确保不会被覆盖，需要进行保存和恢复。



看下是如何跳转的

```c++
// 两个栈进行切换
jump_stack(cur_meta->stack, next_meta->stack);

inline void jump_stack(ContextualStack* from, ContextualStack* to) {
    bthread_jump_fcontext(&from->context, to->context, 0/*not skip remained*/);
}
intptr_t BTHREAD_CONTEXT_CALL_CONVENTION
bthread_jump_fcontext(bthread_fcontext_t * ofc, bthread_fcontext_t nfc,
                      intptr_t vp, bool preserve_fpu = false);
```





不要在pthread上创建前台任务，为什么？





正常pthread都在执行worker_thread函数，里面有个 run_main_task 函数，这个pthread基本就被这个占用了，此时当有新任务给他的时候，也不会是这个pthread新建的，应该是其他pthread来新建的任务





### __thread 原理？









## 参考资料

[Brpc源码浅析](https://blog.csdn.net/zhougb3/category_9761361.html)

[brpc源码解析](https://me.csdn.net/wxj1992)

[高性能RPC框架BRPC核心机制分析<一>](https://zhuanlan.zhihu.com/p/113427004)

[高性能RPC框架BRPC核心机制《二》](https://zhuanlan.zhihu.com/p/142161985)

[brpc源码学习（二）-bthread的创建与切换](https://blog.csdn.net/KIDGIN7439/article/details/106426635)

[brpc源码学习（四）- bthread调度执行总体流程](https://blog.csdn.net/KIDGIN7439/article/details/107837530)

