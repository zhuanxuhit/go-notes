/**
 *@Author:wangchao.zhuanxu
 *@Created At:2020/7/8 9:30 上午
 *@Description:
 */
package activate_object

type MethodRequest int

const (
	Incr MethodRequest = iota
	Decr
)

type Service struct {
	queue chan MethodRequest
	v     int
}

func New(buffer int) *Service {
	s := &Service{
		queue: make(chan MethodRequest, buffer),
	}
	go s.schedule()
	return s
}
func (s *Service) schedule() {
	for r := range s.queue {
		if r == Incr {
			s.v++
		} else {
			s.v--
		}
	}
}
func (s *Service) Incr() {
	s.queue <- Incr
}
func (s *Service) Decr() {
	s.queue <- Decr
}
