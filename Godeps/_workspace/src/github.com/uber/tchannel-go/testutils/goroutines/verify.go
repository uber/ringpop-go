// Copyright (c) 2015 Uber Technologies, Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package goroutines

import (
	"bufio"
	"bytes"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"
)

// isLeak returns whether the given stack contains a stack frame that is considered a leak.
func (s Stack) isLeak() bool {
	isLeakLine := func(line string) bool {
		return strings.Contains(line, "(*Channel).Serve") ||
			strings.Contains(line, "(*Connection).readFrames") ||
			strings.Contains(line, "(*Connection).writeFrames") ||
			strings.Contains(line, "(*Connection).dispatchInbound.func")
	}

	lineReader := bufio.NewReader(bytes.NewReader(s.fullStack.Bytes()))
	for {
		line, err := lineReader.ReadString('\n')
		if err == io.EOF {
			return false
		}
		if err != nil {
			panic(err)
		}

		if isLeakLine(line) {
			return true
		}
	}
}

func getLeakStacks(stacks []Stack) []Stack {
	var leakStacks []Stack
	for _, s := range stacks {
		if s.isLeak() {
			leakStacks = append(leakStacks, s)
		}
	}
	return leakStacks
}

// filterStacks will filter any stacks excluded by the given VerifyOpts.
func filterStacks(stacks []Stack, opts *VerifyOpts) []Stack {
	filtered := stacks[:0]
	for _, stack := range stacks {
		if opts.ShouldSkip(stack) {
			continue
		}
		filtered = append(filtered, stack)
	}
	return filtered
}

// VerifyNoLeaks verifies that there are no goroutines running that are stuck
// inside of readFrames or writeFrames.
// Since some goroutines may still be performing work in the background, we retry the
// checks if any goroutines are fine in a running state a finite number of times.
func VerifyNoLeaks(t *testing.T, opts *VerifyOpts) {
	retryStates := map[string]struct{}{
		"runnable": struct{}{},
		"running":  struct{}{},
		"syscall":  struct{}{},
	}
	const maxAttempts = 50
	var leakAttempts int
	var stacks []Stack

retry:
	for i := 0; i < maxAttempts; i++ {
		// Ignore the first stack which is the current goroutine that is doing the verification.
		stacks = GetAll()[1:]
		stacks = filterStacks(stacks, opts)
		for _, stack := range stacks {
			if _, ok := retryStates[stack.State()]; ok {
				runtime.Gosched()
				if i > maxAttempts/2 {
					time.Sleep(time.Millisecond)
				}
				continue retry
			}
		}

		// There are no running/runnable goroutines, so check for bad leaks.
		leakStacks := getLeakStacks(stacks)

		// If there are leaks found, retry 3 times since the goroutine's state
		// may not yet have updated.
		if len(leakStacks) > 0 && leakAttempts < 3 {
			leakAttempts++
			i--
			continue
		}

		for _, v := range leakStacks {
			t.Errorf("Found leaked goroutine: %v", v)
		}

		// Note: we cannot use NumGoroutine here as it includes system goroutines
		// while runtime.Stack does not: https://github.com/golang/go/issues/11706
		if len(stacks) > 2 {
			t.Errorf("Expect at most 2 goroutines, found more:\n%s", stacks)
		}
		return
	}

	t.Errorf("VerifyNoBlockedGoroutines failed: too many retries. Stacks:\n%s", stacks)
}
