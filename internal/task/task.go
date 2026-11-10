package task

import (
	"fmt"
	"sort"
	"time"
)

type Priority int

const (
	Low Priority = iota
	Medium
	High
)

func (p Priority) String() string {
	switch p {
	case Low:
		return "low"
	case Medium:
		return "medium"
	case High:
		return "high"
	}
	return "unknown"
}

func ParsePriority(s string) (Priority, error) {
	switch s {
	case "low":
		return Low, nil
	case "medium":
		return Medium, nil
	case "high":
		return High, nil
	}
	return 0, fmt.Errorf("invalid priority %q (want low|medium|high)", s)
}

type Status string

const (
	Pending Status = "pending"
	Done    Status = "done"
)

type Task struct {
	ID        int64      `json:"id"`
	Title     string     `json:"title"`
	Priority  Priority   `json:"priority"`
	Status    Status     `json:"status"`
	Tags      []string   `json:"tags,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	DoneAt    *time.Time `json:"done_at,omitempty"`
}

type List []*Task

func (l List) NextID() int64 {
	var max int64
	for _, t := range l {
		if t.ID > max {
			max = t.ID
		}
	}
	return max + 1
}

func (l List) Find(id int64) *Task {
	for _, t := range l {
		if t.ID == id {
			return t
		}
	}
	return nil
}

func (l List) SortByPriority() {
	sort.SliceStable(l, func(i, j int) bool {
		if l[i].Priority != l[j].Priority {
			return l[i].Priority > l[j].Priority
		}
		return l[i].CreatedAt.Before(l[j].CreatedAt)
	})
}

func (l List) Filter(fn func(*Task) bool) List {
	out := make(List, 0, len(l))
	for _, t := range l {
		if fn(t) {
			out = append(out, t)
		}
	}
	return out
}
