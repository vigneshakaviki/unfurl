// Package lint holds the footgun rules unfurl runs against rendered resources.
//
// Rules live in rules.go as plain data + a small check function each, so adding
// a rule is a self-contained PR: append one Rule value, no wiring elsewhere.
package lint

import (
	"sort"

	"github.com/vignesh/unfurl/internal/model"
)

// Severity ranks a finding.
type Severity int

const (
	Low Severity = iota
	Medium
	High
)

func (s Severity) String() string {
	switch s {
	case High:
		return "high"
	case Medium:
		return "medium"
	default:
		return "low"
	}
}

// Rule is one footgun check. Check returns a slice of human messages; each
// message becomes a Finding tagged with the rule's ID/Severity/Fix.
type Rule struct {
	ID       string
	Severity Severity
	// Why is a short static description of the class of problem.
	Why string
	// Fix is the one-line remediation shown to the user.
	Fix string
	// Check inspects a single resource and returns zero or more specific
	// messages (e.g. naming the offending container).
	Check func(r model.Resource) []string
}

// Finding is a rule firing on a specific resource.
type Finding struct {
	RuleID   string
	Severity Severity
	Ref      string // Resource.Ref()
	Message  string
	Fix      string
}

// Run evaluates all rules against all resources and returns findings sorted
// high-severity first, then by resource ref for stable output.
func Run(resources []model.Resource) []Finding {
	var out []Finding
	for _, res := range resources {
		for _, rule := range Rules {
			for _, msg := range rule.Check(res) {
				out = append(out, Finding{
					RuleID:   rule.ID,
					Severity: rule.Severity,
					Ref:      res.Ref(),
					Message:  msg,
					Fix:      rule.Fix,
				})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity > out[j].Severity
		}
		return out[i].Ref < out[j].Ref
	})
	return out
}

// Counts returns how many findings exist at each severity.
func Counts(fs []Finding) (high, medium, low int) {
	for _, f := range fs {
		switch f.Severity {
		case High:
			high++
		case Medium:
			medium++
		case Low:
			low++
		}
	}
	return
}
