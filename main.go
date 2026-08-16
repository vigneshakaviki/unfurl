// unfurl — see what your Helm chart / Kustomize / manifests actually deploy.
//
// Read-only. Renders via the real helm/kustomize tools, then prints a DEPLOYS
// summary, a FOOTGUNS lint report, and (optionally) a plain-English breakdown.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/vigneshakaviki/unfurl/internal/lint"
	"github.com/vigneshakaviki/unfurl/internal/render"
	"github.com/vigneshakaviki/unfurl/internal/report"
)

// version is overridden at build time with -ldflags "-X main.version=...".
var version = "dev"

const usage = `unfurl — see what your Helm chart actually deploys

USAGE:
  unfurl [flags] <path|->
  helm template ... | unfurl -

INPUT (auto-detected):
  ./chart            Helm chart (dir with Chart.yaml)      -> helm template
  ./overlays/prod    Kustomize dir (kustomization.yaml)    -> kustomize build
  manifest.yaml      Rendered manifest file
  ./dir              Dir of .yaml manifests (concatenated)
  -                  Read manifests from stdin

FLAGS:
  -f <file>       values file for Helm (repeatable)
  --release <n>   Helm release name (default "unfurl")
  --explain       add a plain-English breakdown (no AI, offline)
  --lint          footguns only (skip the DEPLOYS panel)
  --json          machine-readable output for CI
  --fail-on <s>   exit 1 if any finding >= severity (high|medium|low)
  --no-color      disable ANSI color
  --version       print version
  -h, --help      this help

Examples:
  unfurl ./charts/api
  unfurl -f values-prod.yaml ./charts/api --explain
  kubectl kustomize ./overlays/prod | unfurl - --fail-on high
`

type flags struct {
	path       string
	valuesFile []string
	release    string
	explain    bool
	lintOnly   bool
	jsonOut    bool
	failOn     string
	noColor    bool
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	f, err := parseArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		fmt.Fprintln(os.Stderr, "\nrun `unfurl --help` for usage")
		return 2
	}
	if f == nil {
		return 0 // help/version already printed
	}

	resources, src, err := render.Render(f.path, render.Options{
		ValuesFiles: f.valuesFile,
		Release:     f.release,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if len(resources) == 0 {
		fmt.Fprintln(os.Stderr, "error: no Kubernetes resources found in input")
		return 1
	}

	findings := lint.Run(resources)

	if f.jsonOut {
		if err := report.WriteJSON(os.Stdout, src, resources, findings); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
	} else {
		report.WriteHuman(os.Stdout, src, resources, findings, report.Options{
			Color:    useColor(f.noColor),
			Explain:  f.explain,
			LintOnly: f.lintOnly,
		})
	}

	if f.failOn != "" && exceeds(findings, f.failOn) {
		return 1
	}
	return 0
}

func parseArgs(args []string) (*flags, error) {
	f := &flags{release: "unfurl"}
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Print(usage)
			return nil, nil
		case a == "--version":
			fmt.Println("unfurl", version)
			return nil, nil
		case a == "--explain":
			f.explain = true
		case a == "--lint":
			f.lintOnly = true
		case a == "--json":
			f.jsonOut = true
		case a == "--no-color":
			f.noColor = true
		case a == "-f" || a == "--values":
			v, err := nextArg(args, &i, a)
			if err != nil {
				return nil, err
			}
			f.valuesFile = append(f.valuesFile, v)
		case a == "--release":
			v, err := nextArg(args, &i, a)
			if err != nil {
				return nil, err
			}
			f.release = v
		case a == "--fail-on":
			v, err := nextArg(args, &i, a)
			if err != nil {
				return nil, err
			}
			if v != "high" && v != "medium" && v != "low" {
				return nil, fmt.Errorf("--fail-on must be high, medium, or low (got %q)", v)
			}
			f.failOn = v
		case strings.HasPrefix(a, "--values="):
			f.valuesFile = append(f.valuesFile, strings.TrimPrefix(a, "--values="))
		case strings.HasPrefix(a, "--fail-on="):
			f.failOn = strings.TrimPrefix(a, "--fail-on=")
		case a == "-":
			f.path = "-"
		case strings.HasPrefix(a, "-") && a != "-":
			return nil, fmt.Errorf("unknown flag %q", a)
		default:
			if f.path != "" {
				return nil, fmt.Errorf("unexpected extra argument %q", a)
			}
			f.path = a
		}
		i++
	}
	if f.path == "" {
		return nil, fmt.Errorf("no input path given (use a path or `-` for stdin)")
	}
	return f, nil
}

func nextArg(args []string, i *int, flag string) (string, error) {
	if *i+1 >= len(args) {
		return "", fmt.Errorf("%s needs a value", flag)
	}
	*i++
	return args[*i], nil
}

func exceeds(findings []lint.Finding, threshold string) bool {
	want := map[string]lint.Severity{"high": lint.High, "medium": lint.Medium, "low": lint.Low}[threshold]
	for _, f := range findings {
		if f.Severity >= want {
			return true
		}
	}
	return false
}

// useColor decides whether to emit ANSI: off if --no-color, NO_COLOR set, or
// stdout is not a terminal (piped/redirected).
func useColor(noColor bool) bool {
	if noColor || os.Getenv("NO_COLOR") != "" {
		return false
	}
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
