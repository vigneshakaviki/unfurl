# Contributing to unfurl

The best PRs are **new footgun rules** and **better resource summaries**. Both
are small, self-contained changes.

## Add a footgun rule

Every rule is one value in [`internal/lint/rules.go`](internal/lint/rules.go).
Append to the `Rules` slice:

```go
{
    ID: "my-check", Severity: Medium,
    Why: "One sentence: what class of problem this is.",
    Fix: "One line: how to fix it.",
    Check: func(r model.Resource) []string {
        if !isWorkload(r) {
            return nil
        }
        var out []string
        for _, c := range r.Containers() {
            // inspect c, append a specific message per hit
        }
        return out
    },
},
```

Rules:

- `Check` returns **one message per specific hit** (name the offending container /
  volume / field). Return `nil` when the rule doesn't apply.
- Pick severity honestly: **high** = security hole or guaranteed breakage,
  **medium** = real reliability/hygiene issue, **low** = nice-to-fix.
- Add a test in [`internal/lint/lint_test.go`](internal/lint/lint_test.go) with a
  tiny manifest that triggers (and ideally one that doesn't).

Helpers available: `isWorkload`, `r.Containers()`, `r.PodTemplateSpec()`,
`r.Get/GetMap/GetSlice/GetString(path...)`, `podSecurityContext`, `boolField`,
`toMapSlice`, `toStrSlice`, `hasWildcard`.

## Add a summary fact

Extend `facts()` in [`internal/report/summary.go`](internal/report/summary.go)
with a `case` for the kind, keeping the line short and scannable.

## Before you open a PR

```sh
make check   # vet + test
```

Keep output terse and scannable — the whole point of unfurl is that you can read
it at a glance.
