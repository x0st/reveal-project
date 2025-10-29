package core

import (
	"context"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/semaphore"
)

type Parallel struct {
	Err            chan error
	sem            *semaphore.Weighted
	wg             sync.WaitGroup
	ctx            context.Context
	someoneErrored atomic.Bool
}

func NewParallel(ctx context.Context, num int) *Parallel {
	if ctx == nil {
		ctx = context.Background()
	}

	return &Parallel{
		ctx: ctx,
		sem: semaphore.NewWeighted(int64(num)),
		Err: make(chan error, num),
	}
}

func (r *Parallel) Schedule(f func() error) {
	r.wg.Add(1)

	go func() {
		defer r.wg.Done()

		err := f()
		if err != nil {
			r.someoneErrored.Store(true)
			r.Err <- err
		}
	}()
}

func (r *Parallel) Wait() error {
	r.wg.Wait()

	select {
	case err := <-r.Err:
		return err
	default:
	}

	return nil
}

func (r *Parallel) Run(f func() error) error {
	if r.someoneErrored.Load() {
		return nil
	}

	r.wg.Add(1)
	err := r.sem.Acquire(r.ctx, 1)
	if err != nil {
		return err
	}

	go func() {
		errF := f()
		if errF != nil {
			r.someoneErrored.Store(true)

			r.wg.Done()
			r.sem.Release(1)

			r.Err <- errF
			return
		}

		r.wg.Done()
		r.sem.Release(1)
	}()

	return nil
}
