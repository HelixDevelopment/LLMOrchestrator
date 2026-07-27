// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package parser

import "testing"

// TestExtractActions_TextOrdering_FollowsTextOrderDeterministically is the
// §11.4.115 RED-baseline regression guard for the nondeterministic
// action-ordering defect in extractActionsFromText.
//
// Root cause: the keyword scan ranged over the package-level
// `actionKeywords` map. Go randomizes map iteration order on every range,
// so the ORDER of the []agent.Action slice returned by ExtractActions —
// the sequence a UI-automation consumer executes — varied run to run and
// did not follow the order the keywords appear in the source text.
//
// In "...click ... then type ..." the click precedes the type, so a
// correct parser MUST always return [click, type]. The guard runs many
// iterations; on the buggy (map-order) implementation at least one run
// yields [type, click] and fails.
func TestExtractActions_TextOrdering_FollowsTextOrderDeterministically(t *testing.T) {
	p := NewParser()
	// "click" appears before "type"; no other action keyword is a
	// substring, so exactly two actions are extracted.
	const raw = "Please click the Login button then type your username here"

	const runs = 64
	for i := 0; i < runs; i++ {
		actions, err := p.ExtractActions(raw)
		if err != nil {
			t.Fatalf("run %d: ExtractActions error: %v", i, err)
		}
		if len(actions) != 2 {
			t.Fatalf("run %d: got %d actions, want 2: %+v", i, len(actions), actions)
		}
		if actions[0].Type != "click" || actions[1].Type != "type" {
			t.Fatalf("run %d: action order = [%s, %s], want [click, type] — "+
				"nondeterministic map-iteration ordering breaks the executed action sequence",
				i, actions[0].Type, actions[1].Type)
		}
	}
}
