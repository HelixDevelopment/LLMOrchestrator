// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Vasic Digital. All rights reserved.

package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"digital.vasic.llmorchestrator/pkg/agent"
)

var (
	// ErrEmptyInput is returned when the input string is empty.
	ErrEmptyInput = errors.New("empty input")
	// ErrNoJSONFound is returned when no JSON block is found in the input.
	ErrNoJSONFound = errors.New("no JSON found in input")
	// ErrMalformedJSON is returned when JSON is found but cannot be parsed.
	ErrMalformedJSON = errors.New("malformed JSON")
	// ErrResponseTooLong is returned when response exceeds maximum safe length.
	ErrResponseTooLong = errors.New("response exceeds maximum length")
)

const (
	// MaxResponseLength is the maximum allowed response length (1MB).
	MaxResponseLength = 1024 * 1024
)

// ResponseParser extracts structured data from raw LLM output.
type ResponseParser interface {
	// Parse extracts a fully parsed response from raw LLM output.
	Parse(raw string) (agent.ParsedResponse, error)
	// ExtractJSON extracts the first JSON object or array from the raw output.
	ExtractJSON(raw string) (map[string]any, error)
	// ExtractActions extracts structured actions from raw output.
	ExtractActions(raw string) ([]agent.Action, error)
	// ExtractIssues extracts issues from raw output.
	ExtractIssues(raw string) ([]agent.Issue, error)
}

// jsonBlockRegex matches JSON code blocks (```json ... ```) or bare JSON objects/arrays.
var jsonBlockRegex = regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(\\{.*?\\}|\\[.*?\\])\\s*```")

// bareJSONObjectRegex matches a bare JSON object not inside a code block.
var bareJSONObjectRegex = regexp.MustCompile(`(?s)\{[^{}]*(?:\{[^{}]*\}[^{}]*)*\}`)

// actionKeywords maps keywords to action types.
var actionKeywords = map[string]string{
	"click":    "click",
	"tap":      "click",
	"press":    "click",
	"type":     "type",
	"enter":    "type",
	"input":    "type",
	"scroll":   "scroll",
	"swipe":    "scroll",
	"navigate": "navigate",
	"go to":    "navigate",
	"open":     "navigate",
	"back":     "back",
	"home":     "home",
}

// DefaultParser is the default ResponseParser implementation.
type DefaultParser struct{}

// NewParser creates a new DefaultParser.
func NewParser() ResponseParser {
	return &DefaultParser{}
}

// Parse extracts a fully parsed response from raw LLM output.
func (p *DefaultParser) Parse(raw string) (agent.ParsedResponse, error) {
	if raw == "" {
		return agent.ParsedResponse{}, ErrEmptyInput
	}

	if len(raw) > MaxResponseLength {
		return agent.ParsedResponse{}, ErrResponseTooLong
	}

	result := agent.ParsedResponse{
		Raw:     raw,
		Content: extractContent(raw),
	}

	// Try to extract JSON.
	jsonData, err := p.ExtractJSON(raw)
	if err == nil {
		result.JSON = jsonData
	}

	// Extract actions.
	actions, err := p.ExtractActions(raw)
	if err == nil {
		result.Actions = actions
	}

	// Extract issues.
	issues, err := p.ExtractIssues(raw)
	if err == nil {
		result.Issues = issues
	}

	return result, nil
}

// ExtractJSON extracts the first JSON object from the raw output.
func (p *DefaultParser) ExtractJSON(raw string) (map[string]any, error) {
	if raw == "" {
		return nil, ErrEmptyInput
	}

	if len(raw) > MaxResponseLength {
		return nil, ErrResponseTooLong
	}

	// First, try code blocks.
	matches := jsonBlockRegex.FindStringSubmatch(raw)
	if len(matches) > 1 {
		var result map[string]any
		if err := json.Unmarshal([]byte(matches[1]), &result); err == nil {
			return result, nil
		}
	}

	// Try to find a bare JSON object.
	bareMatches := bareJSONObjectRegex.FindAllString(raw, -1)
	for _, m := range bareMatches {
		var result map[string]any
		if err := json.Unmarshal([]byte(m), &result); err == nil {
			return result, nil
		}
	}

	// Try the whole string as JSON.
	trimmed := strings.TrimSpace(raw)
	var result map[string]any
	if err := json.Unmarshal([]byte(trimmed), &result); err == nil {
		return result, nil
	}

	return nil, ErrNoJSONFound
}

// ExtractActions extracts structured actions from raw output.
func (p *DefaultParser) ExtractActions(raw string) ([]agent.Action, error) {
	if raw == "" {
		return nil, ErrEmptyInput
	}

	if len(raw) > MaxResponseLength {
		return nil, ErrResponseTooLong
	}

	var actions []agent.Action

	// Try JSON extraction first.
	jsonData, err := p.ExtractJSON(raw)
	if err == nil {
		actions = append(actions, extractActionsFromJSON(jsonData)...)
	}

	// Try to find actions array in JSON.
	allMatches := bareJSONObjectRegex.FindAllString(raw, -1)
	for _, m := range allMatches {
		var obj map[string]any
		if err := json.Unmarshal([]byte(m), &obj); err == nil {
			if actList, ok := obj["actions"]; ok {
				if actArr, ok := actList.([]any); ok {
					for _, a := range actArr {
						if actMap, ok := a.(map[string]any); ok {
							action := parseActionMap(actMap)
							if action.Type != "" {
								actions = append(actions, action)
							}
						}
					}
				}
			}
		}
	}

	// Keyword-based extraction as fallback.
	if len(actions) == 0 {
		actions = extractActionsFromText(raw)
	}

	if len(actions) == 0 {
		return nil, nil
	}

	return deduplicateActions(actions), nil
}

// ExtractIssues extracts issues from raw output.
func (p *DefaultParser) ExtractIssues(raw string) ([]agent.Issue, error) {
	if raw == "" {
		return nil, ErrEmptyInput
	}

	if len(raw) > MaxResponseLength {
		return nil, ErrResponseTooLong
	}

	var issues []agent.Issue

	// Try JSON extraction.
	jsonData, err := p.ExtractJSON(raw)
	if err == nil {
		issues = append(issues, extractIssuesFromJSON(jsonData)...)
	}

	// Look for issue patterns in text.
	textIssues := extractIssuesFromText(raw)
	issues = append(issues, textIssues...)

	if len(issues) == 0 {
		return nil, nil
	}

	return issues, nil
}

// extractContent extracts the main text content from raw output,
// stripping code blocks and metadata.
func extractContent(raw string) string {
	// Remove code blocks.
	noBlocks := regexp.MustCompile("(?s)```.*?```").ReplaceAllString(raw, "")
	return strings.TrimSpace(noBlocks)
}

// extractActionsFromJSON extracts actions from a parsed JSON map.
func extractActionsFromJSON(data map[string]any) []agent.Action {
	var actions []agent.Action

	// Check for "actions" key.
	if actList, ok := data["actions"]; ok {
		if actArr, ok := actList.([]any); ok {
			for _, a := range actArr {
				if actMap, ok := a.(map[string]any); ok {
					action := parseActionMap(actMap)
					if action.Type != "" {
						actions = append(actions, action)
					}
				}
			}
		}
	}

	// Check for single action at top level.
	if actionType, ok := data["type"].(string); ok {
		action := parseActionMap(data)
		if action.Type != "" || actionType != "" {
			if action.Type == "" {
				action.Type = actionType
			}
			actions = append(actions, action)
		}
	}

	return actions
}

// parseActionMap converts a JSON map to an Action.
func parseActionMap(m map[string]any) agent.Action {
	action := agent.Action{}
	if t, ok := m["type"].(string); ok {
		action.Type = t
	}
	if t, ok := m["target"].(string); ok {
		action.Target = t
	}
	if v, ok := m["value"].(string); ok {
		action.Value = v
	}
	if c, ok := m["confidence"].(float64); ok {
		action.Confidence = c
	}
	return action
}

// extractActionsFromText extracts actions from natural language text using keyword matching.
//
// Actions are ordered deterministically by the position at which their
// keyword first appears in the source text. Ranging over the actionKeywords
// map directly would make the returned slice order depend on Go's
// randomized map iteration, so the sequence a UI-automation consumer
// executes would vary run to run and would not follow the order the
// instructions actually appear in the text.
func extractActionsFromText(raw string) []agent.Action {
	lower := strings.ToLower(raw)

	type hit struct {
		pos        int
		actionType string
		target     string
	}
	var hits []hit

	for keyword, actionType := range actionKeywords {
		idx := strings.Index(lower, keyword)
		if idx < 0 {
			continue
		}
		// Extract the target: the text following the keyword up to the next period, comma, or newline.
		after := raw[idx+len(keyword):]
		after = strings.TrimLeft(after, " :\"'")
		endIdx := strings.IndexAny(after, ".,;\n\"'")
		target := after
		if endIdx > 0 {
			target = after[:endIdx]
		}
		target = strings.TrimSpace(target)
		if len(target) > 100 {
			target = target[:100]
		}
		if target != "" {
			hits = append(hits, hit{pos: idx, actionType: actionType, target: target})
		}
	}

	// Deterministic ordering: by first-occurrence position in the text,
	// then by action type as a stable tie-break when two keywords share
	// the same index.
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].pos != hits[j].pos {
			return hits[i].pos < hits[j].pos
		}
		return hits[i].actionType < hits[j].actionType
	})

	actions := make([]agent.Action, 0, len(hits))
	for _, h := range hits {
		actions = append(actions, agent.Action{
			Type:       h.actionType,
			Target:     h.target,
			Confidence: 0.5, // lower confidence for text-extracted actions
		})
	}

	return actions
}

// extractIssuesFromJSON extracts issues from a parsed JSON map.
func extractIssuesFromJSON(data map[string]any) []agent.Issue {
	var issues []agent.Issue

	// Check for "issues" key.
	if issueList, ok := data["issues"]; ok {
		if issueArr, ok := issueList.([]any); ok {
			for _, i := range issueArr {
				if issueMap, ok := i.(map[string]any); ok {
					issue := parseIssueMap(issueMap)
					if issue.Title != "" || issue.Description != "" {
						issues = append(issues, issue)
					}
				}
			}
		}
	}

	// Check for single issue at top level.
	if _, ok := data["severity"]; ok {
		issue := parseIssueMap(data)
		if issue.Title != "" || issue.Description != "" {
			issues = append(issues, issue)
		}
	}

	return issues
}

// parseIssueMap converts a JSON map to an Issue.
func parseIssueMap(m map[string]any) agent.Issue {
	issue := agent.Issue{}
	if t, ok := m["type"].(string); ok {
		issue.Type = t
	}
	if s, ok := m["severity"].(string); ok {
		issue.Severity = s
	}
	if t, ok := m["title"].(string); ok {
		issue.Title = t
	}
	if d, ok := m["description"].(string); ok {
		issue.Description = d
	}
	if s, ok := m["screen_id"].(string); ok {
		issue.ScreenID = s
	}
	if e, ok := m["evidence"].([]any); ok {
		for _, ev := range e {
			if str, ok := ev.(string); ok {
				issue.Evidence = append(issue.Evidence, str)
			}
		}
	}
	return issue
}

// extractIssuesFromText looks for issue-like patterns in natural language text.
func extractIssuesFromText(raw string) []agent.Issue {
	var issues []agent.Issue
	lower := strings.ToLower(raw)

	severityPatterns := map[string]string{
		"critical": "critical",
		"high":     "high",
		"medium":   "medium",
		"low":      "low",
	}

	issueTypePatterns := map[string]string{
		"visual bug":     "visual",
		"ux issue":       "ux",
		"accessibility":  "accessibility",
		"crash":          "crash",
		"performance":    "performance",
		"functional bug": "functional",
	}

	for typePattern, issueType := range issueTypePatterns {
		if strings.Contains(lower, typePattern) {
			severity := "medium" // default
			for sevPattern, sevValue := range severityPatterns {
				if strings.Contains(lower, sevPattern) {
					severity = sevValue
					break
				}
			}

			// Try to extract a description nearby.
			idx := strings.Index(lower, typePattern)
			desc := raw[idx:]
			endIdx := strings.Index(desc, "\n")
			if endIdx > 0 {
				desc = desc[:endIdx]
			}
			if len(desc) > 200 {
				desc = desc[:200]
			}

			issues = append(issues, agent.Issue{
				Type:        issueType,
				Severity:    severity,
				Title:       fmt.Sprintf("Detected %s", typePattern),
				Description: strings.TrimSpace(desc),
			})
		}
	}

	return issues
}

// deduplicateActions removes duplicate actions.
func deduplicateActions(actions []agent.Action) []agent.Action {
	seen := make(map[string]bool)
	var result []agent.Action
	for _, a := range actions {
		key := a.Type + "|" + a.Target + "|" + a.Value
		if !seen[key] {
			seen[key] = true
			result = append(result, a)
		}
	}
	return result
}
