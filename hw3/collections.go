package main

import (
	"fmt"
	"sync"
)

func collections() {
	m := make(map[int]int)
	var wg sync.WaitGroup

	for g := range 50 {
		g := g
		wg.Go(func() {
			for i := 0; i < 1000; i++ {
				m[g*1000+i] = i
			}
		})
	}

	wg.Wait()
	fmt.Println(len(m))
}
