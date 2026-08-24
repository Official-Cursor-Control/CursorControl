//go:build windows

package main

import "sync"

var (
	mainThreadTaskMu sync.Mutex
	mainThreadTasks  []func()
)

// postMainThreadTask schedules state/UI work on the Win32 message thread.
// Network/audio worker goroutines must not mutate authoritative gameplay state
// directly; they should hand the result back through this queue.
func postMainThreadTask(fn func()) {
	if fn == nil {
		return
	}
	mainThreadTaskMu.Lock()
	mainThreadTasks = append(mainThreadTasks, fn)
	mainThreadTaskMu.Unlock()
	if mainHwnd != 0 {
		postMessageW.Call(mainHwnd, WM_MAIN_THREAD_TASK, 0, 0)
	}
}

func postMainThreadTaskAndWait(fn func()) {
	if fn == nil {
		return
	}
	if mainHwnd == 0 {
		fn()
		return
	}
	done := make(chan struct{})
	postMainThreadTask(func() {
		fn()
		close(done)
	})
	<-done
}

func processMainThreadTasks() {
	for {
		mainThreadTaskMu.Lock()
		if len(mainThreadTasks) == 0 {
			mainThreadTaskMu.Unlock()
			return
		}
		tasks := mainThreadTasks
		mainThreadTasks = nil
		mainThreadTaskMu.Unlock()
		for _, fn := range tasks {
			if fn != nil {
				fn()
			}
		}
	}
}
