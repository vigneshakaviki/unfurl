// Package report renders the human and JSON output.
package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/vignesh/unfurl/internal/model"
)

// Item is one line in the DEPLOYS panel.
type Item struct {
	Ref   string // e.g. "Deployment/api"
	Facts string // e.g. "2 replicas · :8080 · ghcr.io/x:1.4.2"
}

// Summarize produces a DEPLOYS line per resource, ordered by kind then name.
func Summarize(resources []model.Resource) []Item {
	items := make([]Item, 0, len(resources))
	for _, r := range resources {
		items = append(items, Item{Ref: r.Ref(), Facts: facts(r)})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Ref < items[j].Ref })
	return items
}

func facts(r model.Resource) string {
	switch r.Kind {
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet":
		return workloadFacts(r)
	case "Pod":
		return joinNonEmpty(" · ", images(r), ports(r))
	case "Job":
		return joinNonEmpty(" · ", "job", images(r))
	case "CronJob":
		return joinNonEmpty(" · ", schedule(r), images(r))
	case "Service":
		return serviceFacts(r)
	case "Ingress":
		return ingressFacts(r)
	case "Secret":
		return keyCount(r, "Secret") + typeSuffix(r)
	case "ConfigMap":
		return keyCount(r, "ConfigMap")
	case "PersistentVolumeClaim":
		return pvcFacts(r)
	case "HorizontalPodAutoscaler":
		return hpaFacts(r)
	case "ServiceAccount", "Role", "ClusterRole", "RoleBinding", "ClusterRoleBinding":
		return strings.ToLower("rbac")
	}
	return ""
}

func workloadFacts(r model.Resource) string {
	parts := []string{}
	if v, ok := model.IntField(r.Get("spec", "replicas")); ok {
		parts = append(parts, fmt.Sprintf("%d replicas", v))
	} else if r.Kind == "DaemonSet" {
		parts = append(parts, "per-node")
	}
	if p := ports(r); p != "" {
		parts = append(parts, p)
	}
	if img := images(r); img != "" {
		parts = append(parts, img)
	}
	return strings.Join(parts, " · ")
}

func images(r model.Resource) string {
	var imgs []string
	for _, c := range r.Containers() {
		if img, ok := c["image"].(string); ok && img != "" {
			imgs = append(imgs, img)
		}
	}
	return strings.Join(dedupe(imgs), ", ")
}

func ports(r model.Resource) string {
	var ps []string
	for _, c := range r.Containers() {
		cp, _ := c["ports"].([]any)
		for _, e := range cp {
			m, _ := e.(map[string]any)
			if v, ok := model.IntField(m["containerPort"]); ok {
				ps = append(ps, fmt.Sprintf(":%d", v))
			}
		}
	}
	return strings.Join(dedupe(ps), " ")
}

func serviceFacts(r model.Resource) string {
	typ, _ := r.Get("spec", "type").(string)
	if typ == "" {
		typ = "ClusterIP"
	}
	var ps []string
	for _, e := range r.GetSlice("spec", "ports") {
		m, _ := e.(map[string]any)
		if v, ok := model.IntField(m["port"]); ok {
			ps = append(ps, fmt.Sprintf("%d", v))
		}
	}
	return joinNonEmpty(" → ", typ, strings.Join(ps, ","))
}

func ingressFacts(r model.Resource) string {
	var hosts []string
	for _, e := range r.GetSlice("spec", "rules") {
		m, _ := e.(map[string]any)
		if h, ok := m["host"].(string); ok && h != "" {
			hosts = append(hosts, h)
		}
	}
	if len(hosts) == 0 {
		return "ingress"
	}
	return strings.Join(hosts, ", ")
}

func keyCount(r model.Resource, _ string) string {
	n := len(r.GetMap("data")) + len(r.GetMap("stringData"))
	if n == 1 {
		return "1 key"
	}
	return fmt.Sprintf("%d keys", n)
}

func typeSuffix(r model.Resource) string {
	if t, ok := r.Get("type").(string); ok && t != "" && t != "Opaque" {
		return " (" + t + ")"
	}
	return ""
}

func pvcFacts(r model.Resource) string {
	size, _ := r.Get("spec", "resources", "requests", "storage").(string)
	return joinNonEmpty(" ", "pvc", size)
}

func hpaFacts(r model.Resource) string {
	minR, _ := model.IntField(r.Get("spec", "minReplicas"))
	maxR, _ := model.IntField(r.Get("spec", "maxReplicas"))
	return fmt.Sprintf("scales %d→%d", minR, maxR)
}

func schedule(r model.Resource) string {
	if s, ok := r.Get("spec", "schedule").(string); ok {
		return s
	}
	return "cron"
}

func joinNonEmpty(sep string, parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, sep)
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
