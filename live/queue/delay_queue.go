package queue

import (
	"context"
	"sync"
	"time"
)

type DelayQueue[T Delayable] struct {
	q             *PriorityQueue[T]
	mutex         *sync.Mutex
	enqueueSignal chan struct{}
	dequeueSignal chan struct{}
	//enqueueSignal *sync.Cond
	//dequeueSignal *sync.Cond
	zero T
}

func NewDelayQueue[T Delayable](c int) *DelayQueue[T] {
	m := &sync.Mutex{}
	res := &DelayQueue[T]{
		q: NewPriorityQueue[T](c, func(src T, dst T) int {
			srcDelay := src.Delay()
			dstDelay := dst.Delay()
			if srcDelay > dstDelay {
				return 1
			}
			if srcDelay == dstDelay {
				return 0
			}
			return -1
		}),
		mutex:         m,
		enqueueSignal: make(chan struct{}, c),
		//dequeueSignal: sync.NewCond(m),
		//enqueueSignal: sync.NewCond(m),
	}
	return res
}

func (d *DelayQueue[T]) In(ctx context.Context, val T) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	d.mutex.Lock()
	for d.q.isFull() {
		d.mutex.Unlock()
		select {
		case <-d.dequeueSignal:
			d.mutex.Lock()
			// 不需要做什么
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	//d.mutex.Lock()
	first, err := d.q.Peek()
	if err != nil {
		d.mutex.Unlock()
		return err
	}
	d.q.Enqueue(val)
	d.mutex.Unlock()
	if val.Delay() < first.Delay() {

		close(d.enqueueSignal)
		//select {
		//case d.enqueueSignal <- struct{}{}:
		//
		//	//default:
		//
		//}

		//d.enqueueSignal.Broadcast()
	}

	return nil
}

// 出队永远拿到"到期"了的
// 如果没有到期的元素，就阻塞，直到有元素到期
// 如果超时了，直接返回

// 先考虑 Out，你会把代码写成什么样子
func (d *DelayQueue[T]) Out(ctx context.Context) (T, error) {
	if ctx.Err() != nil {
		return d.zero, ctx.Err()
	}
	var timer *time.Timer
	for {
		d.mutex.Lock()
		first, err := d.q.Peek()
		d.mutex.Unlock()
		switch err {
		// 你拿到了队首元素
		case nil:
			// 1. delay 是 10s
			delay := first.Delay()
			if delay <= 0 {
				d.mutex.Lock()
				first, err = d.q.Peek()
				if err != nil {
					d.mutex.Unlock()
					continue
				}
				if first.Delay() <= 0 {
					first, err = d.q.Dequeue()
					d.mutex.Unlock()
					return first, err
				}
				d.mutex.Unlock()
			}

			// 这里，delay 还没到期
			if timer == nil {
				timer = time.NewTimer(delay)
			} else {
				timer.Stop()
				timer.Reset(delay)
			}

			//
			select {
			case <-timer.C:
			// 元素到期了，
			// 1. 啥都不干，进入下一个循环
			// 2.
			case <-d.enqueueSignal:
			// 来了新元素
			// 1. 啥都不干，进入下一个循环
			case <-ctx.Done():
				// 超时了
				return d.zero, ctx.Err()
			}

			// 队列里面根本没有元素
		case ErrEmptyQueue:
			// 你要阻塞住自己，等 In 调用，或者等超时
			select {
			case <-d.enqueueSignal:
			// 来了新元素
			// 1. 啥都不干，进入下一个循环
			case <-ctx.Done():
				// 超时了
				return d.zero, ctx.Err()
			}
			// 出错了
		default:
			return d.zero, err
		}
	}
}

type Delayable interface {
	// Delay 实时计算
	Delay() time.Duration
}

type DelayableV1 interface {
	// EndTime 过期的那一刻
	EndTime() time.Time
}

type DelayElem struct {
	end time.Time
}

func (d DelayElem) EndTime() time.Time {
	return d.end
}

func (d DelayElem) Delay() time.Duration {
	return d.end.Sub(time.Now())
}