package testutil

import (
	"bytes"
	"errors"
	"sync"
	"time"
)

// MockClock implements Clock interface for testing with controllable time.
// This is used across multiple rate limiter tests to avoid actual time delays.
type MockClock struct {
	mu  sync.Mutex
	now time.Time
}

// NewMockClock creates a new MockClock starting at the given time.
// If zero time is provided, uses current time.
func NewMockClock(start time.Time) *MockClock {
	if start.IsZero() {
		start = time.Now()
	}
	return &MockClock{now: start}
}

// Now returns the current mock time.
func (m *MockClock) Now() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.now
}

// Advance moves the mock clock forward by the given duration.
func (m *MockClock) Advance(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = m.now.Add(d)
}

// Set sets the mock clock to a specific time.
func (m *MockClock) Set(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = t
}

// MockWriter is a test writer that can simulate various write conditions
// including delays, errors, and write counting.
type MockWriter struct {
	buf         *bytes.Buffer
	mu          sync.Mutex
	writeDelay  time.Duration
	errorOnNth  int
	writeCount  int
	shouldError bool
	err         error
}

// NewMockWriter creates a new MockWriter.
func NewMockWriter() *MockWriter {
	return &MockWriter{
		buf: &bytes.Buffer{},
	}
}

// Write implements io.Writer interface with configurable behavior.
func (mw *MockWriter) Write(p []byte) (int, error) {
	mw.mu.Lock()
	defer mw.mu.Unlock()

	mw.writeCount++

	if mw.writeDelay > 0 {
		time.Sleep(mw.writeDelay)
	}

	if mw.shouldError {
		return 0, mw.err
	}

	if mw.errorOnNth > 0 && mw.writeCount == mw.errorOnNth {
		return 0, errors.New("simulated error")
	}

	return mw.buf.Write(p)
}

// String returns the current buffer contents.
func (mw *MockWriter) String() string {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	return mw.buf.String()
}

// Len returns the current buffer length.
func (mw *MockWriter) Len() int {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	return mw.buf.Len()
}

// WriteCount returns the number of Write calls.
func (mw *MockWriter) WriteCount() int {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	return mw.writeCount
}

// SetWriteDelay configures a delay for each write operation.
func (mw *MockWriter) SetWriteDelay(delay time.Duration) {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	mw.writeDelay = delay
}

// SetErrorOnNth configures the writer to error on the nth write.
func (mw *MockWriter) SetErrorOnNth(n int) {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	mw.errorOnNth = n
}

// SetAlwaysError configures the writer to always return the given error.
func (mw *MockWriter) SetAlwaysError(err error) {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	mw.shouldError = true
	mw.err = err
}

// Reset clears the buffer and resets counters.
func (mw *MockWriter) Reset() {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	mw.buf.Reset()
	mw.writeCount = 0
	mw.shouldError = false
	mw.errorOnNth = 0
	mw.writeDelay = 0
	mw.err = nil
}
