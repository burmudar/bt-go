package peer

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
)

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

type Task struct {
	ID   int
	done chan error
	Fn   func(r *reporter) error
}

type TaskPool struct {
	size     int
	reporter *reporter
	queue    chan *Task
	complete chan error
	stopC    chan bool
	stop     atomic.Bool

	initialized bool
	sync.Mutex
}

func NewTaskPool(size int) *TaskPool {
	return &TaskPool{
		size:     size,
		reporter: newReporter(),
	}
}

func (tp *TaskPool) Init() {
	tp.queue = make(chan *Task, tp.size)
	tp.stopC = make(chan bool)
	tp.reporter.listen(tp.stopC)

	for i := 0; i < tp.size; i++ {
		w := i
		go func() {
			tp.reporter.L("starting runner %d\n", w)
			for {
				select {
				case t := <-tp.queue:
					{
						defer tp.reporter.L("[runner %d] task %d done\n", w, t.ID)
						tp.reporter.L("[runner %d] processing task %d\n", w, t.ID)
						err := t.Fn(tp.reporter)
						if err != nil {
							tp.reporter.L("[runner %d] task %d error: %v\n", w, t.ID, err)
						}
						t.done <- err
					}
				case <-tp.stopC:
					return
				}
			}
		}()
	}

	tp.initialized = true

}

func (tp *TaskPool) Single(t *Task) chan error {
	if !tp.initialized {
		panic("task pool not initialized")
	}
	d := make(chan error)
	go func() {
		select {
		case <-tp.stopC:
			return
		default:
			tp.queue <- t
		}
	}()

	return d

}

func (tp *TaskPool) Close() {
	tp.stop.Store(true)
	close(tp.stopC)
	close(tp.queue)
}

func (tp *TaskPool) Process(tasks []*Task) chan []error {
	if !tp.initialized {
		panic("not initialized")
	}

	completed := make(chan []error)
	done := make(chan error)
	errs := []error{}
	go func() {
		for i, t := range tasks {
			t.done = done
			if tp.stop.Load() {
				return
			}
			select {
			case <-tp.stopC:
				{
					return
				}
			case tp.queue <- t:
				{
					tp.reporter.L("[pool] %d:task-%d <-", i, t.ID)
				}
			}
		}
		fmt.Printf("%d tasks added to pool", len(tasks))
	}()

	go func() {
		count := 0
	loop:
		for {
			select {
			case err := <-done:
				{
					count++
					fmt.Printf("[%d/%d] tasks complete\n", count, len(tasks))
					if err != nil {
						errs = append(errs, err)
					}
					if count == len(tasks) {
						break loop
					}
				}
			case <-tp.stopC:
				{
					break loop
				}
			}
		}
		completed <- errs
	}()

	return completed
}
