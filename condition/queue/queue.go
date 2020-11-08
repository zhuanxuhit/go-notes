package queue

import "sync"

type Queue struct {
	data []interface{}
	capa int
	cond *sync.Cond
}

func New(capacity int) *Queue {
	return &Queue{
		data: make([]interface{}, 0, capacity),
		capa: capacity,
		cond: sync.NewCond(&sync.Mutex{}),
	}
}

func (q *Queue)Enqueue(data interface{}) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	for len(q.data) == q.capa {
		q.cond.Wait()
	}
	q.data = append(q.data, data)
	q.cond.Broadcast()
}
func (q *Queue) Dequeue() (d interface{}) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	for len(q.data) == 0 {
		q.cond.Wait()
	}
	d = q.data[0]
	q.data = q.data[1:]
	q.cond.Broadcast()
	return
}
