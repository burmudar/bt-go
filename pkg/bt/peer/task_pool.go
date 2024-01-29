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
	Done chan error
	Fn   func(r *reporter) error
}

type TaskPool struct {
	size     int
	reporter *reporter
	queue    chan *Task
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
						tp.reporter.L("[runner %d] processing\n", w)
						t.Done <- t.Fn(tp.reporter)
						tp.reporter.L("[runner %d] done\n", w)
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
		t.Done = d
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
			if tp.stop.Load() {
				return
			}
			t.Done = done
			select {
			case <-tp.stopC:
				{
					return
				}
			case tp.queue <- t:
				{
					tp.reporter.L("task %d added", i)
				}
			}
		}
	}()

	go func() {
		for err := range done {
			if err != nil {
				errs = append(errs, err)
			}
		}
		completed <- errs
	}()

	return completed
}
