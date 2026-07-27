// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"testing"
	"time"
)

// TestPipeTransport_Receive_InvalidMessageDoesNotStickyWedge is the LO-1
// §11.4.115 regression guard.
//
// THE DEFECT (pre-fix): the reader loop treats a malformed JSON line as
// RECOVERABLE — it publishes ErrInvalidMessage then `continue`s reading (so
// later valid lines still arrive). But Receive blanket-stickied ANY channel
// error (including the recoverable ErrInvalidMessage) into pt.termErr. Once
// stickied, every subsequent Receive early-returns the stale ErrInvalidMessage
// WITHOUT draining recvCh, so:
//   - the already-staged VALID message is lost forever, and
//   - the reader goroutine blocks forever on the next full-buffer publish
//     (goroutine leak).
//
// Wire fed here: one garbage line, then one valid PipeMessage (Content
// "MSG-OK"), then EOF.
//   - Receive#1 must return a non-terminal error that errors.Is ErrInvalidMessage.
//   - Receive#2 must drain the VALID message (Content == "MSG-OK", nil error),
//     proving the malformed line did NOT sticky-wedge the transport.
//   - After Close()+settle the reader goroutine must return to baseline (the
//     leak guard). In the broken build Receive#2 returns the stale error
//     without draining recvCh, so the reader stays blocked and this never
//     converges.
//
// Receive#2 uses a bounded ctx so a wedge surfaces as a FAILURE, not a hang.
func TestPipeTransport_Receive_InvalidMessageDoesNotStickyWedge(t *testing.T) {
	valid := PipeMessage{Type: MessageTypeResponse, Content: "MSG-OK", RequestID: "req-ok"}
	vb, err := json.Marshal(valid)
	if err != nil {
		t.Fatalf("marshal valid message: %v", err)
	}

	var wire bytes.Buffer
	wire.WriteString("garbage line\n") // malformed → recoverable ErrInvalidMessage
	wire.Write(vb)                     // valid JSON PipeMessage line ...
	wire.WriteByte('\n')               // ... newline-framed, then EOF

	baseline := runtime.NumGoroutine()
	pt := NewPipeTransport(bytes.NewReader(wire.Bytes()), &bytes.Buffer{})

	// Receive#1: malformed line is a RECOVERABLE per-message error, never a
	// wire-terminal error.
	ctx1, cancel1 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel1()
	if _, err1 := pt.Receive(ctx1); err1 == nil {
		t.Fatal("Receive#1: expected an error for the malformed line, got nil")
	} else if !errors.Is(err1, ErrInvalidMessage) {
		t.Fatalf("Receive#1: expected ErrInvalidMessage, got %v", err1)
	}

	// Receive#2: MUST drain the NEXT (valid) message. In the broken
	// (blanket-termErr) build this returns the stale ErrInvalidMessage and
	// MSG-OK is lost. Bounded ctx so a wedge is a failure, not a hang.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	got, err2 := pt.Receive(ctx2)
	if err2 != nil {
		t.Fatalf("Receive#2: transport wedged by non-terminal ErrInvalidMessage — expected the valid message, got error %v", err2)
	}
	if got.Content != "MSG-OK" {
		t.Fatalf("Receive#2: expected the valid message Content=%q, got %q (staged message lost)", "MSG-OK", got.Content)
	}

	// Goroutine-leak guard: with the wire exhausted and the transport closed,
	// the single reader goroutine must exit and the count return to baseline.
	// A permanently-blocked (leaked) reader never lets it converge.
	_ = pt.Close()
	if err := waitGoroutineBaseline(baseline, 2*time.Second); err != nil {
		t.Fatalf("reader goroutine leaked after invalid-message recovery: %v", err)
	}
}

// waitGoroutineBaseline polls until runtime.NumGoroutine() returns to at most
// baseline (allowing short-lived goroutines to settle) or the timeout elapses.
func waitGoroutineBaseline(baseline int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var n int
	for time.Now().Before(deadline) {
		n = runtime.NumGoroutine()
		if n <= baseline {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fmt.Errorf("goroutine count %d did not return to baseline %d within %s", n, baseline, timeout)
}
