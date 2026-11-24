package main

import (
	"fmt"
	"sync"
	"time"
)

type JobStatus struct {
	status bool
	fn     func()
}

type JobMonitor struct {
	lock         sync.Mutex
	jobs         map[string]JobStatus
	wg           sync.WaitGroup
	timeInterval time.Duration
}

func newJobMonitor() *JobMonitor {
	return &JobMonitor{
		jobs:         make(map[string]JobStatus),
		wg:           sync.WaitGroup{},
		timeInterval: 30 * time.Second,
	}
}

func (m *JobMonitor) register(name string, fn func()) {
	fnc := func() {
		defer func() {
			job, ok := m.jobs[name]

			if !ok {
				fmt.Println("Job not restart as not exist in the monitor")
			}

			job.status = false

			m.lock.Lock()
			m.jobs[name] = job
			m.wg.Done()
			m.lock.Unlock()
		}()

		fn()
	}

	job := JobStatus{
		status: true,
		fn:     fnc,
	}

	m.lock.Lock()
	m.jobs[name] = job
	m.lock.Unlock()
}

func a() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Recover From Panic: %s\n", r.(string))
		}
	}()

	for i := 1; i <= 5; i++ {
		fmt.Println("A Running")
		time.Sleep(1 * time.Second)
	}

	panic("A Panic")
}

func main() {
	mon := newJobMonitor()
	mon.register("a", a)

	mon.wg.Wait()
}
