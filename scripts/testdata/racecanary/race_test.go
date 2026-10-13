package racecanary

import (
	"sync"
	"testing"
)

func TestRealRace(t *testing.T) {
	var x int
	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 100000; j++ {
				x++
			}
		}()
	}
	wg.Wait()
	_ = x
}
