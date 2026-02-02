package main

import (
	"sync"
	"time"
)

type SafeMap struct {
	mu sync.Mutex
	m  map[int]int
}

func (s *SafeMap) Set(k, v int) {
	s.mu.Lock()
	s.m[k] = v
	s.mu.Unlock()
}

func mutexCollections() float64 {
	sm := &SafeMap{m: make(map[int]int)}
	var wg sync.WaitGroup
	start := time.Now()

	for g := 0; g < 50; g++ {
		g := g
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				sm.Set(g*1000+i, i)
			}
		}()
	}

	wg.Wait()
	return float64(time.Since(start).Nanoseconds()) / 1e6
}