package report

import (
	"fmt"
	"strings"

	"github.com/vigneshakaviki/unfurl/internal/model"
)

// Explain returns a plain-English, rule-based description of a workload/service
// stack. No AI: deterministic sentences built from the parsed spec. Returns an
// empty string for resources we don't narrate.
func Explain(resources []model.Resource) string {
	var lines []string
	for _, r := range resources {
		if s := explainOne(r); s != "" {
			lines = append(lines, s)
		}
	}
	return strings.Join(lines, "\n")
}

func explainOne(r model.Resource) string {
	switch r.Kind {
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet":
		return explainWorkload(r)
	case "Service":
		typ, _ := r.Get("spec", "type").(string)
		if typ == "" {
			typ = "ClusterIP"
		}
		return fmt.Sprintf("• %s exposes traffic (%s) to matching pods.", r.Ref(), typ)
	case "Ingress":
		hosts := ingressFacts(r)
		return fmt.Sprintf("• %s publishes the service to the outside world at %s.", r.Ref(), hosts)
	case "CronJob":
		return fmt.Sprintf("• %s runs a scheduled job (%s).", r.Ref(), schedule(r))
	}
	return ""
}

func explainWorkload(r model.Resource) string {
	replicas := "some"
	if v, ok := model.IntField(r.Get("spec", "replicas")); ok {
		replicas = fmt.Sprintf("%d", v)
	} else if r.Kind == "DaemonSet" {
		replicas = "one-per-node"
	}
	img := images(r)
	port := ports(r)

	s := fmt.Sprintf("• %s runs %s cop%s", r.Ref(), replicas, plural(replicas))
	if img != "" {
		s += " of " + img
	}
	if port != "" {
		s += ", listening on " + port
	}
	s += "."

	// Add the single most important caveat inline so --explain is actionable.
	if lacksLimits(r) {
		s += " No resource limits — it can starve the node under load."
	}
	return s
}

func plural(replicas string) string {
	if replicas == "1" {
		return "y" // "cop" + "y"
	}
	return "ies" // "cop" + "ies"
}

func lacksLimits(r model.Resource) bool {
	for _, c := range r.Containers() {
		res, _ := c["resources"].(map[string]any)
		lim, _ := res["limits"].(map[string]any)
		if len(lim) == 0 {
			return true
		}
	}
	return false
}
