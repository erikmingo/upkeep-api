// Package rules loads the YAML rule library, validates it against the fact taxonomy,
// stores it in the rules table, and decides which rules a home's facts satisfy.
package rules

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/erikmingo/upkeep-api/internal/db"
	"github.com/goccy/go-yaml"
)

//go:embed rules.yaml
var source []byte

type Rule struct {
	ID   string           `yaml:"id" json:"id"`
	When map[string][]any `yaml:"when" json:"when"`
	Task Task             `yaml:"task" json:"task"`
}

type Task struct {
	Title    string  `yaml:"title" json:"title"`
	Interval int32   `yaml:"interval" json:"interval"`
	Detail   string  `yaml:"detail,omitempty" json:"detail,omitempty"`
	Season   *Season `yaml:"season,omitempty" json:"season,omitempty"`
}

type Season struct {
	Start string `yaml:"start" json:"start"`
	End   string `yaml:"end" json:"end"`
}

// Parse returns the embedded library, or the first validation error.
func Parse() ([]Rule, error) { return parse(source) }

func parse(src []byte) ([]Rule, error) {
	var rs []Rule
	if err := yaml.Unmarshal(src, &rs); err != nil {
		return nil, fmt.Errorf("rules.yaml: %w", err)
	}
	seen := map[string]bool{}
	for _, r := range rs {
		if r.ID == "" || r.Task.Title == "" || r.Task.Interval <= 0 {
			return nil, fmt.Errorf("rule %q: id, task.title and task.interval > 0 are required", r.ID)
		}
		if seen[r.ID] {
			return nil, fmt.Errorf("rule %q: duplicate id", r.ID)
		}
		seen[r.ID] = true
		if se := r.Task.Season; se != nil && (len(se.Start) != 5 || len(se.End) != 5) {
			return nil, fmt.Errorf("rule %q: season start/end must be MM-DD, got %q %q", r.ID, se.Start, se.End)
		}
		for key, allowed := range r.When {
			tax, ok := Taxonomy[key]
			if !ok {
				return nil, fmt.Errorf("rule %q: unknown fact key %q", r.ID, key)
			}
			if len(allowed) == 0 {
				return nil, fmt.Errorf("rule %q: %q lists no values", r.ID, key)
			}
			for _, v := range allowed {
				if !permitted(tax, v) {
					return nil, fmt.Errorf("rule %q: %v is not a valid value for %q", r.ID, v, key)
				}
			}
		}
	}
	return rs, nil
}

func permitted(tax []any, v any) bool {
	if k, isKind := tax[0].(kind); isKind {
		switch k {
		case kindBool:
			_, ok := v.(bool)
			return ok
		case kindInt:
			_, ok := v.(int)
			if !ok {
				_, ok = v.(uint64)
			}
			return ok
		case kindString:
			_, ok := v.(string)
			return ok
		}
	}
	for _, t := range tax {
		if reflect.DeepEqual(t, v) {
			return true
		}
	}
	return false
}

// Load parses the library and upserts every rule into the rules table.
func Load(ctx context.Context, q *db.Queries) ([]Rule, error) {
	rs, err := Parse()
	if err != nil {
		return nil, err
	}
	for _, r := range rs {
		def, err := json.Marshal(r)
		if err != nil {
			return nil, err
		}
		if _, err := q.UpsertRule(ctx, db.UpsertRuleParams{ID: r.ID, Version: 1, Definition: def}); err != nil {
			return nil, fmt.Errorf("rule %q: %w", r.ID, err)
		}
	}
	return rs, nil
}

// Facts is a home's facts as key → decoded JSON value (string, bool, float64).
type Facts map[string]any

// FactsFrom decodes fact rows; values arrive as JSON so numbers are float64.
func FactsFrom(rows []db.Fact) (Facts, error) {
	f := make(Facts, len(rows))
	for _, r := range rows {
		var v any
		if err := json.Unmarshal(r.Value, &v); err != nil {
			return nil, fmt.Errorf("fact %s: %w", r.Key, err)
		}
		f[r.Key] = v
	}
	return f, nil
}

// Matches reports whether every `when` key has a fact whose value is among the allowed ones.
func (r Rule) Matches(f Facts) bool {
	for key, allowed := range r.When {
		v, ok := f[key]
		if !ok {
			return false
		}
		hit := false
		for _, a := range allowed {
			if same(a, v) {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	return true
}

func same(ruleValue, factValue any) bool {
	switch rv := ruleValue.(type) {
	case int:
		fv, ok := factValue.(float64)
		return ok && float64(rv) == fv
	case uint64:
		fv, ok := factValue.(float64)
		return ok && float64(rv) == fv
	}
	return reflect.DeepEqual(ruleValue, factValue)
}

// Matching returns the rules a home's facts satisfy, in library order.
func Matching(rs []Rule, f Facts) []Rule {
	var out []Rule
	for _, r := range rs {
		if r.Matches(f) {
			out = append(out, r)
		}
	}
	return out
}
