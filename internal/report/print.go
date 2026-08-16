package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/vigneshakaviki/unfurl/internal/lint"
	"github.com/vigneshakaviki/unfurl/internal/model"
	"github.com/vigneshakaviki/unfurl/internal/render"
)

// ANSI helpers. Disabled by setting color=false (no-tty / --no-color / NO_COLOR).
type palette struct{ on bool }

func (p palette) wrap(code, s string) string {
	if !p.on {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}
func (p palette) bold(s string) string   { return p.wrap("1", s) }
func (p palette) dim(s string) string    { return p.wrap("2", s) }
func (p palette) red(s string) string    { return p.wrap("31", s) }
func (p palette) yellow(s string) string { return p.wrap("33", s) }
func (p palette) green(s string) string  { return p.wrap("32", s) }
func (p palette) cyan(s string) string   { return p.wrap("36", s) }

// Options for a human render.
type Options struct {
	Color   bool
	Explain bool
	LintOnly bool
}

// WriteHuman prints the DEPLOYS / FOOTGUNS / (optional) PLAIN ENGLISH panels.
func WriteHuman(w io.Writer, src render.Source, resources []model.Resource, findings []lint.Finding, opts Options) {
	p := palette{on: opts.Color}

	if !opts.LintOnly {
		fmt.Fprintf(w, "%s  %s\n\n", p.dim("unfurl"), p.dim(fmt.Sprintf("%s · %s · %d resources", src.Kind, src.Path, len(resources))))

		fmt.Fprintln(w, p.bold("📦 DEPLOYS"))
		items := Summarize(resources)
		refW := maxRefWidth(items)
		for _, it := range items {
			fmt.Fprintf(w, "  %s  %s\n", p.cyan(pad(it.Ref, refW)), p.dim(it.Facts))
		}
		fmt.Fprintln(w)
	}

	high, med, low := lint.Counts(findings)
	header := fmt.Sprintf("⚠️  FOOTGUNS  (%s · %s · %s)",
		p.red(fmt.Sprintf("%d high", high)),
		p.yellow(fmt.Sprintf("%d medium", med)),
		p.dim(fmt.Sprintf("%d low", low)),
	)
	fmt.Fprintln(w, p.bold(header))
	if len(findings) == 0 {
		fmt.Fprintln(w, "  "+p.green("clean — no footguns found"))
	}
	refW := maxFindingWidth(findings)
	for _, f := range findings {
		fmt.Fprintf(w, "  %s %s  %s  %s %s\n",
			sevIcon(f.Severity),
			sevTag(p, f.Severity),
			p.cyan(pad(f.Ref, refW)),
			f.Message,
			p.dim("→ "+f.Fix),
		)
	}

	if opts.Explain && !opts.LintOnly {
		if s := Explain(resources); s != "" {
			fmt.Fprintln(w)
			fmt.Fprintln(w, p.bold("🧠 PLAIN ENGLISH"))
			for _, line := range strings.Split(s, "\n") {
				fmt.Fprintln(w, "  "+line)
			}
		}
	}
}

// jsonOut is the machine-readable shape (stable for CI consumers).
type jsonOut struct {
	Source struct {
		Kind string `json:"kind"`
		Path string `json:"path"`
	} `json:"source"`
	Resources []jsonResource `json:"resources"`
	Findings  []jsonFinding  `json:"findings"`
	Summary   struct {
		High   int `json:"high"`
		Medium int `json:"medium"`
		Low    int `json:"low"`
	} `json:"summary"`
}

type jsonResource struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Facts     string `json:"facts,omitempty"`
}

type jsonFinding struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Ref      string `json:"ref"`
	Message  string `json:"message"`
	Fix      string `json:"fix"`
}

// WriteJSON emits the machine-readable report.
func WriteJSON(w io.Writer, src render.Source, resources []model.Resource, findings []lint.Finding) error {
	var out jsonOut
	out.Source.Kind = src.Kind
	out.Source.Path = src.Path
	for _, it := range Summarize(resources) {
		// Split "Kind/Name" back for structured output.
		kind, name := it.Ref, ""
		if i := strings.IndexByte(it.Ref, '/'); i >= 0 {
			kind, name = it.Ref[:i], it.Ref[i+1:]
		}
		out.Resources = append(out.Resources, jsonResource{Kind: kind, Name: name, Facts: it.Facts})
	}
	for _, f := range findings {
		out.Findings = append(out.Findings, jsonFinding{
			Rule: f.RuleID, Severity: f.Severity.String(), Ref: f.Ref, Message: f.Message, Fix: f.Fix,
		})
	}
	out.Summary.High, out.Summary.Medium, out.Summary.Low = lint.Counts(findings)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func sevIcon(s lint.Severity) string {
	switch s {
	case lint.High:
		return "🔴"
	case lint.Medium:
		return "🟡"
	default:
		return "⚪"
	}
}

func sevTag(p palette, s lint.Severity) string {
	switch s {
	case lint.High:
		return p.red("high  ")
	case lint.Medium:
		return p.yellow("medium")
	default:
		return p.dim("low   ")
	}
}

func pad(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

func maxRefWidth(items []Item) int {
	w := 0
	for _, it := range items {
		if len(it.Ref) > w {
			w = len(it.Ref)
		}
	}
	return w
}

func maxFindingWidth(fs []lint.Finding) int {
	w := 0
	for _, f := range fs {
		if len(f.Ref) > w {
			w = len(f.Ref)
		}
	}
	return w
}
