// Copyright 2026 Chris Wells <chris@rhza.org>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package progress

import (
	"sync"
	"time"
)

// Bus is the event distribution mechanism. The pipeline calls Emit for each
// stage transition; consumers receive events from the channel returned by
// Events. The bus is safe for concurrent use — multiple goroutines (workers)
// may call Emit simultaneously.
//
// Design principles:
//   - Non-blocking sends: if the buffer is full, the event is dropped. The
//     pipeline must never stall on a slow consumer. Correctness lives in the
//     database and ledger; the bus is for observation only.
//   - Unidirectional: events flow pipeline → consumer. Consumers never send
//     commands back through the bus. Pipeline control uses context.Context.
//   - Safe close: Close may be called multiple times without panicking.
type Bus struct {
	ch     chan Event
	once   sync.Once
	closed chan struct{}
}

// NewBus creates a Bus with the given buffer size. A buffer of 256 is
// recommended — large enough to absorb bursts from concurrent workers
// without blocking, small enough to bound memory usage.
func NewBus(bufferSize int) *Bus {
	if bufferSize <= 0 {
		bufferSize = 256
	}
	return &Bus{
		ch:     make(chan Event, bufferSize),
		closed: make(chan struct{}),
	}
}

// Emit sends an event to the bus. The event's Timestamp is set to the
// current time before sending. If the buffer is full or the bus is closed,
// the event is silently dropped — Emit never blocks.
func (b *Bus) Emit(e Event) {
	// Check closed without holding a lock — a closed channel read is always
	// ready, so this is a cheap non-blocking check.
	select {
	case <-b.closed:
		return // bus is closed; drop silently.
	default:
	}

	e.Timestamp = time.Now()

	select {
	case b.ch <- e:
	default:
		// Buffer full — drop the event. The pipeline must not block.
	}
}

// Events returns the receive-only channel for consumers to range over.
// The channel is closed when Close is called, causing any range loop to exit.
func (b *Bus) Events() <-chan Event {
	return b.ch
}

// Close signals that no more events will be emitted. The event channel is
// closed, causing consumers ranging over Events() to exit their loop.
// Close is idempotent — calling it multiple times is safe.
func (b *Bus) Close() {
	b.once.Do(func() {
		close(b.closed)
		close(b.ch)
	})
}
