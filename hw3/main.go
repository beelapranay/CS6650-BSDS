package main

import (
	"flag"
	"fmt"
)

func main() {
	mode := flag.String("mode", "mutex", "mutex|rwmutex|syncmap|atomic|file")
	flag.Parse()

	switch *mode {
	case "mutex":
		fmt.Printf("%.6f\n", mutexCollections())
	case "rwmutex":
		fmt.Printf("%.6f\n", rwmutexCollections())
	case "syncmap":
		fmt.Printf("%.6f\n", syncMapCollections())

	case "atomic":
		atomicCounter() 

	case "file":
		fileAccessExperiment()
	
	case "context":
		contextSwitchExperiment()

	default:
		panic("unknown mode: " + *mode)
	}
}
