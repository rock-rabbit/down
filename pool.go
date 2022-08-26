package down

import (
	"math"
	"sync"
)

// WaitGroupPool sync.WaitGroup 池
type WaitGroupPool struct {
	done chan error
	pool chan struct{}
	wg   *sync.WaitGroup
}

// NewWaitGroupPool 创建一个 size 大小的 sync.WaitGroup 池
func NewWaitGroupPool(size int) *WaitGroupPool {
	if size <= 0 {
		size = math.MaxInt32
	}
	return &WaitGroupPool{
		done: make(chan error, size),
		pool: make(chan struct{}, size),
		wg:   &sync.WaitGroup{},
	}
}

// Count 未完成线程的个数
func (p *WaitGroupPool) Count() int {
	return len(p.pool)
}

// Add 添加一个 sync.WaitGroup 线程
func (p *WaitGroupPool) Add() {
	p.pool <- struct{}{}
	p.wg.Add(1)
}

// Done 完成一个 sync.WaitGroup 线程
func (p *WaitGroupPool) Done() {
	<-p.pool
	p.wg.Done()
}

// Wait 阻塞等待 sync.WaitGroup 清零
func (p *WaitGroupPool) Wait() {
	p.wg.Wait()
}

// Syne 非阻塞等待
func (p *WaitGroupPool) Syne() {
	go func() {
		p.wg.Wait()
		p.done <- nil
	}()
}

// Error 出现错误
func (p *WaitGroupPool) Error(err error) {
	p.done <- err
}

// SyneDone 非阻塞等待时全部完成
func (p *WaitGroupPool) AllDone() <-chan error {
	return p.done
}
