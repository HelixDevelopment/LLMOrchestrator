// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package agent

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// TestSimpleAgentPool_Acquire_DoesNotOverProvisionCapacity is the
// §11.4.115 RED-baseline regression guard for the capacity
// over-provisioning race in SimpleAgentPool.Acquire.
//
// Root cause: Acquire drops p.mu while it runs the (slow) ClientBuilder,
// but the "capacity available?" decision that authorised the build did
// NOT reserve the slot. Two concurrent Acquire calls therefore both read
// len(inUse)+len(available) < capacity == true, both unlock, and both
// build — handing out MORE live agents than the configured capacity.
// Each over-provisioned agent is a real provider process / SDK client /
// network connection beyond the ceiling the pool exists to enforce.
//
// The guard drives capacity=1 with several concurrent Acquire callers and
// a builder that blocks inside the build window, recording the maximum
// number of builder invocations that were ever in flight simultaneously.
// A correct pool never runs more than `capacity` builders concurrently.
func TestSimpleAgentPool_Acquire_DoesNotOverProvisionCapacity(t *testing.T) {
	const capacity = 1
	const callers = 4

	var (
		concurrent    atomic.Int32
		maxConcurrent atomic.Int32
	)
	entered := make(chan struct{}, callers)
	release := make(chan struct{})

	builder := func(ctx context.Context) (Agent, error) {
		cur := concurrent.Add(1)
		for {
			m := maxConcurrent.Load()
			if cur <= m || maxConcurrent.CompareAndSwap(m, cur) {
				break
			}
		}
		entered <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
		concurrent.Add(-1)
		return newMockAgent(fmt.Sprintf("ov-%d", cur), "opencode"), nil
	}

	pool := NewSimpleAgentPool("opencode", capacity, builder)

	results := make(chan error, callers)
	for i := 0; i < callers; i++ {
		go func() {
			a, err := pool.Acquire(context.Background(), AgentRequirements{})
			if err == nil && a != nil {
				pool.Release(a)
			}
			results <- err
		}()
	}

	// Wait for the first builder to enter its (blocked) build window.
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("no builder invocation observed within 2s")
	}

	// Give any concurrent (over-provisioning) builders a window to also
	// enter. A correct pool NEVER admits a second concurrent builder at
	// capacity=1, so nothing else should arrive here.
	time.Sleep(200 * time.Millisecond)

	close(release)
	for i := 0; i < callers; i++ {
		select {
		case err := <-results:
			if err != nil {
				t.Errorf("Acquire returned error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("an Acquire caller did not return within 2s")
		}
	}

	if got := maxConcurrent.Load(); int(got) > capacity {
		t.Fatalf("capacity=%d pool ran %d builders concurrently — capacity over-provisioning race: "+
			"each concurrent builder is one live provider process/connection beyond the configured ceiling",
			capacity, got)
	}
}
