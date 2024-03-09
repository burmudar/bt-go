package types

import "testing"

func TestQueueAddAndPope(t *testing.T) {
	tt := []struct {
		name     string
		items    []int
		expected []int
	}{
		{
			"add 4 items, pop 2",
			[]int{1, 2, 3, 4},
			[]int{1, 2},
		},
		{
			"add 4 items, pop 4",
			[]int{1, 2, 3, 4},
			[]int{1, 2, 3, 4},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			queue := NewSliceQueue[int]()
			for _, i := range tc.items {
				queue.Add(i)
			}

			if queue.Size() != len(tc.items) {
				t.Fatalf("queue has incorrect size %d expected %d", queue.Size(), len(tc.items))
			}

			for i := 0; i < len(tc.expected); i++ {
				v, ok := queue.Pop()

				if !ok {
					t.Errorf("expected more items to be popped but pop returned false")
				}

				if v != tc.expected[i] {
					t.Fatalf("expected %d got %d", tc.expected[i], v)
				}
			}
		})
	}
}

func TestQueueHeadAndPop(t *testing.T) {
	queue := NewSliceQueue[int]()

	if _, ok := queue.Head(); ok {
		t.Fatalf("head should return false if there is not head")
	}

	if _, ok := queue.Pop(); ok {
		t.Fatalf("head should return false if there are not items")
	}

	queue.Add(1)
	queue.Add(2)
	queue.Add(3)
	queue.Add(4)

	for i := 0; i < queue.Size(); i++ {
		if h, ok := queue.Head(); ok {
			v, _ := queue.Pop()
			if h != v {
				t.Fatalf("Head and Popped values are not equal. %d != %d", h, v)
			}
		}
	}
}

func TestQueueTailAndPop(t *testing.T) {
	queue := NewSliceQueue[int]()

	if _, ok := queue.Tail(); ok {
		t.Fatalf("tail should return false if there is not tail")
	}

	queue.Add(1)
	queue.Add(2)
	queue.Add(3)
	queue.Add(4)

	expectedTail := 4
	for i := 0; i < queue.Size()-1; i++ {
		if tail, ok := queue.Tail(); ok {
			if tail != expectedTail {
				t.Fatalf("expected tail to stay the same while head items are removed")
			}
			queue.Pop()
		}
	}
}
