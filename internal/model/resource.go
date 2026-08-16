// Package model holds the parsed representation of a Kubernetes resource.
package model

import "fmt"

// Resource is one Kubernetes object parsed from a rendered manifest stream.
type Resource struct {
	APIVersion string
	Kind       string
	Name       string
	Namespace  string
	// Raw is the fully-parsed YAML document, used by lint rules to inspect
	// arbitrary fields without every rule re-parsing the source.
	Raw map[string]any
}

// Ref is a short human reference like "Deployment/api".
func (r Resource) Ref() string {
	if r.Kind == "" {
		return r.Name
	}
	return fmt.Sprintf("%s/%s", r.Kind, r.Name)
}

// Get walks a path of string keys into the raw document and returns the value
// at that path, or nil if any segment is missing or not a map.
func (r Resource) Get(path ...string) any {
	var cur any = r.Raw
	for _, key := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[key]
	}
	return cur
}

// GetMap is Get with a map assertion.
func (r Resource) GetMap(path ...string) map[string]any {
	m, _ := r.Get(path...).(map[string]any)
	return m
}

// GetSlice is Get with a slice assertion.
func (r Resource) GetSlice(path ...string) []any {
	s, _ := r.Get(path...).([]any)
	return s
}

// GetString is Get with a string assertion.
func (r Resource) GetString(path ...string) string {
	s, _ := r.Get(path...).(string)
	return s
}

// PodTemplateSpec returns the pod spec for workloads that embed one
// (Deployment, StatefulSet, DaemonSet, ReplicaSet, Job), or nil.
func (r Resource) PodTemplateSpec() map[string]any {
	switch r.Kind {
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet":
		return r.GetMap("spec", "template", "spec")
	case "Job":
		return r.GetMap("spec", "template", "spec")
	case "CronJob":
		return r.GetMap("spec", "jobTemplate", "spec", "template", "spec")
	case "Pod":
		return r.GetMap("spec")
	}
	return nil
}

// Containers returns all containers (regular + init) in a pod-bearing workload.
func (r Resource) Containers() []map[string]any {
	ps := r.PodTemplateSpec()
	if ps == nil {
		return nil
	}
	var out []map[string]any
	for _, key := range []string{"initContainers", "containers"} {
		for _, c := range toMaps(ps[key]) {
			out = append(out, c)
		}
	}
	return out
}

func toMaps(v any) []map[string]any {
	s, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(s))
	for _, e := range s {
		if m, ok := e.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// IntField reads an integer-ish field (YAML may decode as int or float64).
func IntField(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}
