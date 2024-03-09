package peer

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"go.uber.org/multierr"
)

var ErrPoolStopped error = fmt.Errorf("pool is stopped")
var ErrPoolProcessing error = fmt.Errorf("pool is processing")

type reporter struct {
	c         chan string
	listening bool
	sync.Mutex
}

func newReporter() *reporter {
	return &reporter{
		c: make(chan string),
	}
}

func (r *reporter) L(s string, vars ...any) {
	if !r.listening {
		return
	}
	r.c <- fmt.Sprintf(s, vars...)
}

func (r *reporter) close() {
	r.Lock()
	r.listening = false
	defer r.Unlock()
	close(r.c)

}

func (r *reporter) listen(stop chan bool) {
	r.Lock()
	defer r.Unlock()
	if r.listening {
		return
	}
	r.listening = true
	debug := os.Getenv("DEBUG") == "1"
	go func() {
		for {
			select {
			case s := <-r.c:
				{
					if debug {
						fmt.Println(s)
					}
				}
			case <-stop:
				{
					r.Lock()
					r.listening = false
					r.Unlock()
					return
				}
			}
		}
	}()
}

type Task[T any] struct {
	ID   int
	done chan *TaskResult[T]
	Fn   func(r *reporter) (T, error)
}

type TaskResult[T any] struct {
	TaskID int
	Err    error
	Result T
}

type TaskPool[T any] struct {
	size       int
	reporter   *reporter
	tasks      []*Task[T]
	queueCh    chan *Task[T]
	complete   chan []*TaskResult[T]
	stopC      chan bool
	processing atomic.Bool
	stopped    atomic.Bool

	init sync.Once
	sync.Mutex
}

func NewTaskPool[T any](size int) *TaskPool[T] {
	return &TaskPool[T]{
		size:     size,
		reporter: newReporter(),
	}
}

func (tp *TaskPool[T]) Add(t *Task[T]) bool {
	if tp.processing.Load() {
		return false
	}

	tp.tasks = append(tp.tasks, t)
	return true
}

func (tp *TaskPool[T]) Init() {
	tp.init.Do(func() {
		tp.queueCh = make(chan *Task[T], tp.size)
		tp.stopC = make(chan bool)
		tp.complete = make(chan []*TaskResult[T])
		tp.reporter.listen(tp.stopC)

		for i := 0; i < tp.size; i++ {
			w := i
			go func() {
				tp.reporter.L("starting runner %d\n", w)
				for {
					select {
					case t := <-tp.queueCh:
						{
							defer tp.reporter.L("[runner %d] task %d done\n", w, t.ID)
							tp.reporter.L("[runner %d] processing task %d\n", w, t.ID)
							result, err := t.Fn(tp.reporter)
							if err != nil {
								tp.reporter.L("[runner %d] task %d error: %v\n", w, t.ID, err)
							}
							t.done <- &TaskResult[T]{
								TaskID: t.ID,
								Err:    err,
								Result: result,
							}
						}
					case <-tp.stopC:
						return
					}
				}
			}()
		}
	})
}

func (tp *TaskPool[T]) Close() {
	close(tp.stopC)
	close(tp.queueCh)
	close(tp.complete)
	tp.stopped.Store(true)
}

func (tp *TaskPool[T]) AwaitComplete(ctx context.Context) ([]T, error) {
	if tp.stopped.Load() {
		return nil, ErrPoolStopped
	}
	for {
		select {
		case results := <-tp.complete:
			{
				values := []T{}
				var err error
				for _, r := range results {
					values = append(values, r.Result)
					err = multierr.Append(err, r.Err)
				}
				return values, err
			}
		case <-ctx.Done():
			{
				return nil, ctx.Err()
			}
		}
	}
}

func (tp *TaskPool[T]) Process() error {
	if tp.stopped.Load() {
		return ErrPoolStopped
	}
	if tp.processing.Load() {
		return ErrPoolProcessing
	}
	tp.Init()
	done := make(chan *TaskResult[T])
	go func() {
		defer tp.reporter.L("[pool(tasks)] stopped")
		for i, t := range tp.tasks {
			t.done = done
			select {
			case <-tp.stopC:
				{
					return
				}
			case tp.queueCh <- t:
				{
					tp.reporter.L("[pool(tasks)] %d task added", i, t.ID)
				}
			}
		}
		fmt.Printf("%d tasks added to pool", len(tp.tasks))
	}()

	go func() {
		count := 0
		results := []*TaskResult[T]{}
		defer tp.reporter.L("[pool(done)] stopped")

		taskCount := len(tp.tasks)
	loop:
		for {
			select {
			case result := <-done:
				{
					count++
					fmt.Printf("[%d/%d] tasks complete\n", count, taskCount)
					results = append(results, result)
					if count == taskCount {
						break loop
					}
				}
			case <-tp.stopC:
				{
					break loop
				}
			}
		}
		tp.complete <- results
	}()
	return nil
}
