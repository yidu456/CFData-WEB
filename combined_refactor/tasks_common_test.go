package main

import (
	"context"
	"testing"
	"testing/synctest"
)

func TestBoundedWorkersWaitsForProgressCallbacks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		entered := make(chan struct{})
		release := make(chan struct{})
		done := make(chan struct{})
		go func() {
			runBoundedWorkers(context.Background(), 1, 1, 1, func(int, int) {
				close(entered)
				<-release
			}, func(int) {})
			close(done)
		}()
		<-entered
		synctest.Wait()
		select {
		case <-done:
			close(release)
			t.Fatal("worker helper returned before its progress callback finished")
		default:
		}
		close(release)
		<-done
	})
}

func TestBoundedWorkersCancellationDrainsScheduledCallbacks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		entered := make(chan struct{})
		release := make(chan struct{})
		done := make(chan bool, 1)
		go func() {
			done <- runBoundedWorkers(ctx, 3, 1, 1, func(current, total int) {
				if current == 1 {
					close(entered)
					<-release
				}
			}, func(int) { cancel() })
		}()
		<-entered
		synctest.Wait()
		select {
		case <-done:
			close(release)
			t.Fatal("cancellation returned before scheduled callback drained")
		default:
		}
		close(release)
		<-done
	})
}
