package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"syscall"
	"time"
)

// // // // // // // // // //

const (
	eventQueueCapacity = 256
	eventDrainTimeout  = time.Second
)

// ErrSlowConsumer means the bounded event queue could not accept an event.
var ErrSlowConsumer = errors.New("event consumer is too slow")

// WriterObj serializes versioned events through one bounded writer goroutine.
type WriterObj struct {
	dst    io.WriteCloser
	now    func() time.Time
	cancel func()

	mu       sync.Mutex
	queue    chan EventObj
	done     chan struct{}
	failed   chan struct{}
	failOnce sync.Once
	closed   bool
	seq      uint64
	err      error
}

// NewWriter starts an event writer that owns dst until Close returns.
func NewWriter(dst io.WriteCloser, now func() time.Time, cancel func()) *WriterObj {
	if now == nil {
		now = time.Now
	}
	if cancel == nil {
		cancel = func() {}
	}
	obj := &WriterObj{
		dst: dst, now: now, cancel: cancel,
		queue: make(chan EventObj, eventQueueCapacity), done: make(chan struct{}),
		failed: make(chan struct{}),
	}
	go obj.run()
	return obj
}

func (obj *WriterObj) fail(err error) {
	obj.mu.Lock()
	if obj.err == nil {
		obj.err = err
	}
	obj.mu.Unlock()
	obj.failOnce.Do(func() { close(obj.failed) })
	obj.cancel()
}

func (obj *WriterObj) run() {
	defer close(obj.done)
	encoder := json.NewEncoder(obj.dst)
	encoder.SetEscapeHTML(false)
	for event := range obj.queue {
		if err := encoder.Encode(event); err != nil {
			obj.fail(fmt.Errorf("write event stream: %w", err))
			return
		}
	}
}

// Enqueue stamps and accepts one event without allowing unbounded buffering.
func (obj *WriterObj) Enqueue(event EventObj) error {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	if obj.closed {
		if obj.err != nil {
			return obj.err
		}
		return errors.New("event writer is closed")
	}
	if obj.err != nil {
		return obj.err
	}

	obj.seq++
	event.Schema = SchemaVersion
	event.Seq = obj.seq
	event.Time = obj.now().UTC().Format(time.RFC3339Nano)
	select {
	case obj.queue <- event:
		return nil
	default:
		obj.err = ErrSlowConsumer
		obj.failOnce.Do(func() { close(obj.failed) })
		obj.cancel()
		return obj.err
	}
}

// Failed is closed when writing fails or the bounded queue fills.
func (obj *WriterObj) Failed() <-chan struct{} {
	return obj.failed
}

// Err returns the first stream failure, if any.
func (obj *WriterObj) Err() error {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	return obj.err
}

// Close drains accepted events and closes the owned destination.
func (obj *WriterObj) Close() error {
	obj.mu.Lock()
	if !obj.closed {
		obj.closed = true
		close(obj.queue)
	}
	obj.mu.Unlock()

	timer := time.NewTimer(eventDrainTimeout)
	defer timer.Stop()
	select {
	case <-obj.done:
	case <-timer.C:
		_ = obj.dst.Close()
		<-obj.done
	}
	if err := obj.dst.Close(); err != nil && !IsBrokenPipe(err) {
		obj.fail(fmt.Errorf("close event stream: %w", err))
	}
	return obj.Err()
}

// IsBrokenPipe reports whether a downstream consumer closed its pipe.
func IsBrokenPipe(err error) bool {
	return errors.Is(err, syscall.EPIPE) || errors.Is(err, io.ErrClosedPipe)
}
