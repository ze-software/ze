// Design: docs/architecture/appliance/remote-operations.md -- bounded worker pool for parallel fleet operations

package appliance

import (
	"fmt"
	"os"
	"sync"
)

const (
	minParallel = 1
	maxParallel = 64
)

type deviceResult struct {
	Name string
	Code int
}

func runParallel(names []string, parallel int, op func(name string) int) int {
	if parallel < minParallel {
		parallel = minParallel
	}
	if parallel > maxParallel {
		parallel = maxParallel
	}
	if parallel > len(names) {
		parallel = len(names)
	}

	if parallel == 1 {
		return runSequential(names, op)
	}

	work := make(chan string, len(names))
	for _, name := range names {
		work <- name
	}
	close(work)

	results := make(chan deviceResult, len(names))
	var wg sync.WaitGroup

	for range parallel {
		wg.Go(func() {
			for name := range work {
				code := op(name)
				results <- deviceResult{Name: name, Code: code}
			}
		})
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	succeeded, failed := 0, 0
	for r := range results {
		if r.Code != exitOK {
			fmt.Fprintf(os.Stderr, "FAILED: %s\n", r.Name)
			failed++
		} else {
			succeeded++
		}
	}

	fmt.Printf("%d succeeded, %d failed\n", succeeded, failed)
	if failed > 0 {
		return exitError
	}
	return exitOK
}

func runSequential(names []string, op func(name string) int) int {
	succeeded, failed := 0, 0
	for _, name := range names {
		if code := op(name); code != exitOK {
			fmt.Fprintf(os.Stderr, "FAILED: %s\n", name)
			failed++
		} else {
			succeeded++
		}
	}

	fmt.Printf("%d succeeded, %d failed\n", succeeded, failed)
	if failed > 0 {
		return exitError
	}
	return exitOK
}
