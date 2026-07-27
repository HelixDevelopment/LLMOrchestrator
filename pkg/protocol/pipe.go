// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package protocol

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

var (
	// ErrTransportClosed is returned when operations are attempted on a closed transport.
	ErrTransportClosed = errors.New("transport is closed")
	// ErrWriteFailed is returned when writing to the pipe fails.
	ErrWriteFailed = errors.New("write to pipe failed")
	// ErrReadFailed is returned when reading from the pipe fails.
	ErrReadFailed = errors.New("read from pipe failed")
	// ErrInvalidMessage is returned when a message fails to parse.
	ErrInvalidMessage = errors.New("invalid message format")
)

// recvResult carries one decoded message (or terminal error) from the
// single background reader goroutine to a Receive caller.
type recvResult struct {
	msg PipeMessage
	err error
}

// PipeTransport manages JSON-lines communication over stdin/stdout pipes.
type PipeTransport struct {
	mu      sync.Mutex
	reader  *bufio.Reader
	writer  io.Writer
	closed  bool
	encoder *json.Encoder

	// recvCh is fed by a single persistent reader goroutine (started lazily
	// by the first Receive). Decoupling the wire read from the per-call
	// Receive is what makes Receive cancellable WITHOUT losing or
	// mis-ordering messages: when a Receive is cancelled its goroutine no
	// longer reads-and-discards from the shared reader — the message simply
	// stays buffered in recvCh and is delivered to the NEXT Receive in order.
	recvCh     chan recvResult
	readerOnce sync.Once

	// termErr stores the terminal read error (io.EOF or ErrReadFailed) once
	// the wire is exhausted, so every Receive after the stream ends keeps
	// returning that error instead of blocking forever on an empty channel
	// (the reader goroutine publishes the terminal error exactly once and
	// then stops). Guarded by mu.
	termErr error
}

// NewPipeTransport creates a new PipeTransport from reader and writer.
func NewPipeTransport(reader io.Reader, writer io.Writer) *PipeTransport {
	return &PipeTransport{
		reader:  bufio.NewReader(reader),
		writer:  writer,
		encoder: json.NewEncoder(writer),
		// Buffer of 1 so the reader can stage the next message without
		// blocking even while no Receive is currently waiting; FIFO order
		// across Receive calls is preserved because exactly one reader
		// goroutine feeds the channel in wire order.
		recvCh: make(chan recvResult, 1),
	}
}

// startReaderLoop launches the single persistent reader goroutine exactly
// once. It reads framed JSON-lines messages in wire order and publishes each
// onto recvCh; on a terminal read error (incl. io.EOF) it publishes the
// error and stops. Because there is exactly ONE reader, messages are never
// duplicated, never reordered, and never silently dropped when a Receive
// caller's context is cancelled.
func (pt *PipeTransport) startReaderLoop() {
	pt.readerOnce.Do(func() {
		go func() {
			for {
				line, err := pt.reader.ReadBytes('\n')
				if err != nil {
					// Deliver any trailing data before the terminal error so
					// a final unterminated message is not lost.
					if len(line) > 0 {
						var msg PipeMessage
						if uerr := json.Unmarshal(line, &msg); uerr == nil {
							pt.recvCh <- recvResult{msg: msg}
						}
					}
					if err == io.EOF {
						pt.recvCh <- recvResult{err: io.EOF}
					} else {
						pt.recvCh <- recvResult{err: fmt.Errorf("%w: %v", ErrReadFailed, err)}
					}
					return
				}
				var msg PipeMessage
				if uerr := json.Unmarshal(line, &msg); uerr != nil {
					pt.recvCh <- recvResult{err: fmt.Errorf("%w: %v", ErrInvalidMessage, uerr)}
					continue
				}
				pt.recvCh <- recvResult{msg: msg}
			}
		}()
	})
}

// Send writes a PipeMessage as a JSON line.
func (pt *PipeTransport) Send(ctx context.Context, msg PipeMessage) error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if pt.closed {
		return ErrTransportClosed
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	msg.Timestamp = time.Now()

	if err := pt.encoder.Encode(msg); err != nil {
		return fmt.Errorf("%w: %v", ErrWriteFailed, err)
	}

	return nil
}

// Receive reads and parses a single JSON-line message.
//
// Receive is cancellable via ctx WITHOUT losing or reordering messages: the
// wire is drained by a single persistent reader goroutine that publishes
// messages in order onto an internal channel. If ctx is cancelled before a
// message arrives, Receive returns ctx.Err() and the not-yet-delivered
// message stays buffered for the NEXT Receive call — it is never consumed
// and discarded (the pre-fix message-loss defect).
func (pt *PipeTransport) Receive(ctx context.Context) (PipeMessage, error) {
	pt.mu.Lock()
	closed := pt.closed
	pt.mu.Unlock()
	if closed {
		return PipeMessage{}, ErrTransportClosed
	}

	if err := ctx.Err(); err != nil {
		return PipeMessage{}, err
	}

	// Sticky terminal error: once the wire is exhausted, every Receive
	// returns the same terminal error (matching the pre-fix behaviour where
	// each call re-hit EOF) rather than blocking on an empty channel.
	pt.mu.Lock()
	te := pt.termErr
	pt.mu.Unlock()
	if te != nil {
		return PipeMessage{}, te
	}

	pt.startReaderLoop()

	select {
	case <-ctx.Done():
		return PipeMessage{}, ctx.Err()
	case r := <-pt.recvCh:
		// LO-1: only WIRE-TERMINAL errors (io.EOF once the stream is
		// exhausted, or ErrReadFailed) are sticky — they mean the wire is
		// permanently dead, so every later Receive keeps returning them
		// (matching termErr's doc above). A per-message parse error
		// (ErrInvalidMessage) is RECOVERABLE: the reader loop publishes it
		// then `continue`s reading, so Receive surfaces it to THIS caller but
		// leaves termErr nil, letting the NEXT Receive drain the next (valid)
		// message — aligning Receive with the reader loop's continue.
		// Blanket-stickying ANY error wedged the transport on one malformed
		// line, silently dropped every later valid message, and leaked the
		// reader goroutine (blocked on the never-drained channel).
		if r.err != nil && (errors.Is(r.err, io.EOF) || errors.Is(r.err, ErrReadFailed)) {
			pt.mu.Lock()
			pt.termErr = r.err
			pt.mu.Unlock()
		}
		return r.msg, r.err
	}
}

// SendPrompt is a convenience method for sending a prompt message.
func (pt *PipeTransport) SendPrompt(ctx context.Context, requestID, content string, imagePath string) error {
	msg := PipeMessage{
		Type:      MessageTypePrompt,
		Content:   content,
		ImagePath: imagePath,
		RequestID: requestID,
	}
	return pt.Send(ctx, msg)
}

// SendShutdown sends a shutdown message.
func (pt *PipeTransport) SendShutdown(ctx context.Context) error {
	msg := PipeMessage{
		Type: MessageTypeShutdown,
	}
	return pt.Send(ctx, msg)
}

// Close marks the transport as closed.
func (pt *PipeTransport) Close() error {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.closed = true
	return nil
}

// IsClosed returns true if the transport has been closed.
func (pt *PipeTransport) IsClosed() bool {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	return pt.closed
}
