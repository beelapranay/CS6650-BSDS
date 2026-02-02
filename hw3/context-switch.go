package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

const rounds = 1_000_000

func pingPongOnce() time.Duration {
	ch := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	start := time.Now()

	go func() {
		defer wg.Done()
		for range rounds {
			ch <- struct{}{}
			<-ch
		}
	}()

	go func() {
		defer wg.Done()
		for range rounds {
			<-ch
			ch <- struct{}{}
		}
	}()

	wg.Wait()
	return time.Since(start)
}

func contextSwitchExperiment() {
	// 1 OS thread
	runtime.GOMAXPROCS(1)
	d1 := pingPongOnce()
	avg1 := d1.Seconds() / float64(2*rounds) // seconds per handoff

	// default (use all cores)
	runtime.GOMAXPROCS(runtime.NumCPU())
	d2 := pingPongOnce()
	avg2 := d2.Seconds() / float64(2*rounds)

	fmt.Println("GOMAXPROCS=1 total:", d1, "avg per handoff:", time.Duration(avg1*1e9), "ns")
	fmt.Println("GOMAXPROCS=all total:", d2, "avg per handoff:", time.Duration(avg2*1e9), "ns")
}
