package main

import (
	"bufio"
	"fmt"
	"os"
	"time"
)

const N = 100000

func writeUnbuffered(path string) time.Duration {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	start := time.Now()
	for i := 0; i < N; i++ {
		_, err := f.Write([]byte("hello\n"))
		if err != nil {
			panic(err)
		}
	}
	return time.Since(start)
}

func writeBuffered(path string) time.Duration {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)

	start := time.Now()
	for i := 0; i < N; i++ {
		_, err := w.WriteString("hello\n")
		if err != nil {
			panic(err)
		}
	}
	if err := w.Flush(); err != nil {
		panic(err)
	}
	return time.Since(start)
}

func fileAccessExperiment() {
	d1 := writeUnbuffered("out_unbuffered.txt")
	d2 := writeBuffered("out_buffered.txt")

	fmt.Println("unbuffered:", d1)
	fmt.Println("buffered:  ", d2)
}