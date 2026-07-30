package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/voluminor/dji-rc-joystick/mod/rc"
)

// // // // // // // // // //

type bufferCloserObj struct {
	bytes.Buffer
	closed bool
}

func (obj *bufferCloserObj) Close() error {
	obj.closed = true
	return nil
}

type pipeWriterObj struct{}

func (pipeWriterObj) Write([]byte) (int, error) {
	return 0, syscall.EPIPE
}

func (pipeWriterObj) Close() error {
	return nil
}

type blockingWriterObj struct {
	once    sync.Once
	started chan struct{}
	release chan struct{}
}

func newBlockingWriter() *blockingWriterObj {
	return &blockingWriterObj{started: make(chan struct{}), release: make(chan struct{})}
}

func (obj *blockingWriterObj) Write(data []byte) (int, error) {
	obj.once.Do(func() { close(obj.started) })
	<-obj.release
	return len(data), nil
}

func (obj *blockingWriterObj) Close() error {
	obj.once.Do(func() { close(obj.started) })
	select {
	case <-obj.release:
	default:
		close(obj.release)
	}
	return nil
}

// //

func TestWriterProducesOrderedVersionedEvents(t *testing.T) {
	destination := &bufferCloserObj{}
	now := func() time.Time {
		return time.Date(2026, 7, 27, 12, 34, 56, 789, time.FixedZone("test", 2*60*60))
	}
	writer := NewWriter(destination, now, nil)
	if err := writer.Enqueue(StatusEvent(RemoteObj{IP: "192.0.2.1"}, rc.StatusObj{
		Phase: rc.PhaseConnected, LinksOnline: 2, LinksTarget: 3,
	})); err != nil {
		t.Fatal(err)
	}
	if err := writer.Enqueue(StateEvent(RemoteObj{IP: "192.0.2.1"}, rc.StateObj{})); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if !destination.closed {
		t.Fatal("writer did not close its destination")
	}

	decoder := json.NewDecoder(bytes.NewReader(destination.Bytes()))
	var first, second EventObj
	if err := decoder.Decode(&first); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&second); err != nil {
		t.Fatal(err)
	}
	if first.Schema != SchemaVersion || first.Seq != 1 || first.Type != "status" {
		t.Fatalf("unexpected first event: %+v", first)
	}
	if second.Schema != SchemaVersion || second.Seq != 2 || second.Type != "state" {
		t.Fatalf("unexpected second event: %+v", second)
	}
	if first.Time != "2026-07-27T10:34:56.000000789Z" || second.Time != first.Time {
		t.Fatalf("unexpected UTC timestamps: %q, %q", first.Time, second.Time)
	}
}

func TestWriterReportsBrokenPipe(t *testing.T) {
	writer := NewWriter(pipeWriterObj{}, time.Now, nil)
	if err := writer.Enqueue(EventObj{Type: "status"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-writer.Failed():
	case <-time.After(time.Second):
		t.Fatal("writer did not report EPIPE")
	}
	err := writer.Close()
	if !IsBrokenPipe(err) {
		t.Fatalf("writer error = %v, want broken pipe", err)
	}
}

func TestWriterFailsBoundedlyForSlowConsumer(t *testing.T) {
	destination := newBlockingWriter()
	canceled := make(chan struct{}, 1)
	writer := NewWriter(destination, time.Now, func() {
		select {
		case canceled <- struct{}{}:
		default:
		}
	})

	if err := writer.Enqueue(EventObj{Type: "state"}); err != nil {
		t.Fatal(err)
	}
	<-destination.started
	var enqueueErr error
	for index := 0; index < eventQueueCapacity+1; index++ {
		enqueueErr = writer.Enqueue(EventObj{Type: "state"})
		if enqueueErr != nil {
			break
		}
	}
	if !errors.Is(enqueueErr, ErrSlowConsumer) {
		t.Fatalf("enqueue error = %v, want slow consumer", enqueueErr)
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("slow consumer did not cancel the session")
	}

	start := time.Now()
	err := writer.Close()
	if !errors.Is(err, ErrSlowConsumer) {
		t.Fatalf("close error = %v, want slow consumer", err)
	}
	if elapsed := time.Since(start); elapsed > 1500*time.Millisecond {
		t.Fatalf("bounded close took %s", elapsed)
	}
}
