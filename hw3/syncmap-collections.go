package main

import (
	"sync"
	"time"
)

func syncMapCollections() float64 {
	var m sync.Map
	var wg sync.WaitGroup
	start := time.Now()

	for g := 0; g < 50; g++ {
		g := g
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				m.Store(g*1000+i, i)
			}
		}()
	}

	wg.Wait()
	return float64(time.Since(start).Nanoseconds()) / 1e6
}