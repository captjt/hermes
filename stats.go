package main

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/fetchbot"
)

// runMemStats controls the debugging and memory allocation statistics
func runMemStats(f *fetchbot.Fetcher, tick time.Duration) {
	var mu sync.Mutex
	var di *fetchbot.DebugInfo

	// Start goroutine to collect fetchbot debug info
	go func() {
		for v := range f.Debug() {
			mu.Lock()
			di = v
			mu.Unlock()
		}
	}()
	// Start ticker goroutine to print mem stats at regular intervals
	go func() {
		c := time.Tick(tick)
		for _ = range c {
			mu.Lock()
			printMemStats(di)
			mu.Unlock()
		}
	}()
}

// printMemStats prints out the memory profile of the application at a given time's state
func printMemStats(di *fetchbot.DebugInfo) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	buf := bytes.NewBuffer(nil)
	buf.WriteString(strings.Repeat("=", 72) + "\n")
	buf.WriteString("Memory Profile:\n")
	buf.WriteString(fmt.Sprintf("\tAlloc: %d Kb\n", mem.Alloc/1024))
	buf.WriteString(fmt.Sprintf("\tTotalAlloc: %d Kb\n", mem.TotalAlloc/1024))
	buf.WriteString(fmt.Sprintf("\tNumGC: %d\n", mem.NumGC))
	buf.WriteString(fmt.Sprintf("\tGoroutines: %d\n", runtime.NumGoroutine()))
	if di != nil {
		buf.WriteString(fmt.Sprintf("\tNumHosts: %d\n", di.NumHosts))
	}
	buf.WriteString(strings.Repeat("=", 72))
	fmt.Println(buf.String())
}
