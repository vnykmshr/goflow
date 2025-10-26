package testutil

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestEventually(t *testing.T) {
	t.Run("condition met immediately", func(t *testing.T) {
		called := false
		Eventually(t, func() bool {
			called = true
			return true
		}, 100*time.Millisecond, 10*time.Millisecond)

		if !called {
			t.Error("condition function should be called")
		}
	})

	t.Run("condition met after delay", func(t *testing.T) {
		var counter int32
		go func() {
			time.Sleep(50 * time.Millisecond)
			atomic.StoreInt32(&counter, 1)
		}()

		Eventually(t, func() bool {
			return atomic.LoadInt32(&counter) == 1
		}, 200*time.Millisecond, 10*time.Millisecond)
	})
}

func TestWaitForInt32(t *testing.T) {
	var value int32

	go func() {
		time.Sleep(30 * time.Millisecond)
		atomic.StoreInt32(&value, 42)
	}()

	WaitForInt32(t, &value, 42, 200*time.Millisecond)

	if atomic.LoadInt32(&value) != 42 {
		t.Errorf("value = %d, want 42", value)
	}
}

func TestWaitForInt64(t *testing.T) {
	var value int64

	go func() {
		time.Sleep(30 * time.Millisecond)
		atomic.StoreInt64(&value, 100)
	}()

	WaitForInt64(t, &value, 100, 200*time.Millisecond)

	if atomic.LoadInt64(&value) != 100 {
		t.Errorf("value = %d, want 100", value)
	}
}

func TestAssertEventually(t *testing.T) {
	var flag bool

	go func() {
		time.Sleep(50 * time.Millisecond)
		flag = true
	}()

	AssertEventually(t, func() bool {
		return flag
	})

	if !flag {
		t.Error("flag should be true")
	}
}

func TestCallbackTracker(t *testing.T) {
	t.Run("basic tracking", func(t *testing.T) {
		tracker := NewCallbackTracker()

		if tracker.Called() {
			t.Error("tracker should not be called initially")
		}

		tracker.Mark()

		if !tracker.Called() {
			t.Error("tracker should be called after Mark()")
		}

		if tracker.CallCount() != 1 {
			t.Errorf("call count = %d, want 1", tracker.CallCount())
		}
	})

	t.Run("multiple calls", func(t *testing.T) {
		tracker := NewCallbackTracker()

		tracker.Mark()
		tracker.Mark()
		tracker.Mark()

		if tracker.CallCount() != 3 {
			t.Errorf("call count = %d, want 3", tracker.CallCount())
		}
	})

	t.Run("value tracking", func(t *testing.T) {
		tracker := NewCallbackTracker()

		tracker.Mark("first")
		if tracker.Value() != "first" {
			t.Errorf("value = %v, want first", tracker.Value())
		}

		tracker.Mark("second")
		if tracker.Value() != "second" {
			t.Errorf("value = %v, want second", tracker.Value())
		}
	})

	t.Run("reset", func(t *testing.T) {
		tracker := NewCallbackTracker()

		tracker.Mark("test")
		tracker.Reset()

		if tracker.Called() {
			t.Error("tracker should not be called after reset")
		}
		if tracker.CallCount() != 0 {
			t.Errorf("call count = %d, want 0", tracker.CallCount())
		}
		if tracker.Value() != nil {
			t.Errorf("value = %v, want nil", tracker.Value())
		}
	})

	t.Run("concurrent access", func(t *testing.T) {
		tracker := NewCallbackTracker()

		const goroutines = 10
		const callsPerGoroutine = 100

		done := make(chan bool, goroutines)
		for i := 0; i < goroutines; i++ {
			go func() {
				for j := 0; j < callsPerGoroutine; j++ {
					tracker.Mark()
				}
				done <- true
			}()
		}

		for i := 0; i < goroutines; i++ {
			<-done
		}

		expected := goroutines * callsPerGoroutine
		if tracker.CallCount() != expected {
			t.Errorf("call count = %d, want %d", tracker.CallCount(), expected)
		}
	})
}

func TestCallbackTrackerAssertions(t *testing.T) {
	t.Run("AssertCalled success", func(t *testing.T) {
		tracker := NewCallbackTracker()
		tracker.Mark()
		tracker.AssertCalled(t)
	})

	t.Run("AssertNotCalled success", func(t *testing.T) {
		tracker := NewCallbackTracker()
		tracker.AssertNotCalled(t)
	})

	t.Run("AssertCallCount success", func(t *testing.T) {
		tracker := NewCallbackTracker()
		tracker.Mark()
		tracker.Mark()
		tracker.AssertCallCount(t, 2)
	})
}

func TestEventuallyWithContext(t *testing.T) {
	t.Run("condition met", func(t *testing.T) {
		var flag bool
		ctx := context.Background()

		go func() {
			time.Sleep(30 * time.Millisecond)
			flag = true
		}()

		EventuallyWithContext(t, ctx, func() bool {
			return flag
		}, 10*time.Millisecond)
	})
}

func TestWithTimeout(t *testing.T) {
	ctx, cancel := WithTimeout(t)
	defer cancel()

	if ctx == nil {
		t.Fatal("context should not be nil")
	}

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("context should have a deadline")
	}

	if time.Until(deadline) > TestTimeout {
		t.Errorf("deadline is too far in the future")
	}
}

func TestAssertNoError(t *testing.T) {
	AssertNoError(t, nil)
}

func TestAssertError(t *testing.T) {
	AssertError(t, context.Canceled)
}

func TestAssertEqual(t *testing.T) {
	AssertEqual(t, 42, 42)
	AssertEqual(t, "hello", "hello")
	AssertEqual(t, true, true)
}

func TestAssertNotEqual(t *testing.T) {
	AssertNotEqual(t, 1, 2)
	AssertNotEqual(t, "a", "b")
	AssertNotEqual(t, true, false)
}
