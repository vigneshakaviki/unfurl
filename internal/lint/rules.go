package lint

import (
	"fmt"
	"strings"

	"github.com/vignesh/unfurl/internal/model"
)

// isWorkload reports whether the resource embeds a pod template we can inspect.
func isWorkload(r model.Resource) bool {
	switch r.Kind {
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job", "CronJob", "Pod":
		return true
	}
	return false
}

// containerName returns a container's name, or "<unnamed>".
func containerName(c map[string]any) string {
	if n, ok := c["name"].(string); ok && n != "" {
		return n
	}
	return "<unnamed>"
}

// podSecurityContext returns the pod-level securityContext (may be nil).
func podSecurityContext(r model.Resource) map[string]any {
	ps := r.PodTemplateSpec()
	if ps == nil {
		return nil
	}
	sc, _ := ps["securityContext"].(map[string]any)
	return sc
}

// boolField reads a bool that may be absent.
func boolField(m map[string]any, key string) (val, present bool) {
	v, ok := m[key]
	if !ok {
		return false, false
	}
	b, _ := v.(bool)
	return b, true
}

// Rules is the full footgun set. Append here to add a rule.
var Rules = []Rule{
	// ---- Reliability ----
	{
		ID: "no-resource-limits", Severity: High,
		Why: "Container has no resource limits; it can starve neighbours or the node.",
		Fix: "set resources.limits (or a LimitRange) for cpu/memory",
		Check: func(r model.Resource) []string {
			if !isWorkload(r) {
				return nil
			}
			var out []string
			for _, c := range r.Containers() {
				res, _ := c["resources"].(map[string]any)
				lim, _ := res["limits"].(map[string]any)
				if len(lim) == 0 {
					out = append(out, fmt.Sprintf("container %q has no resource limits", containerName(c)))
				}
			}
			return out
		},
	},
	{
		ID: "no-resource-requests", Severity: Medium,
		Why: "Container has no resource requests; the scheduler can't place it well and it's un-right-sizable.",
		Fix: "set resources.requests for cpu/memory",
		Check: func(r model.Resource) []string {
			if !isWorkload(r) {
				return nil
			}
			var out []string
			for _, c := range r.Containers() {
				res, _ := c["resources"].(map[string]any)
				req, _ := res["requests"].(map[string]any)
				if len(req) == 0 {
					out = append(out, fmt.Sprintf("container %q has no resource requests", containerName(c)))
				}
			}
			return out
		},
	},
	{
		ID: "cpu-limit-set", Severity: Low,
		Why: "A CPU limit throttles the pod via CFS even when the node has spare CPU, adding latency.",
		Fix: "consider dropping cpu limits (keep requests); see the 'stop using CPU limits' debate",
		Check: func(r model.Resource) []string {
			if !isWorkload(r) {
				return nil
			}
			var out []string
			for _, c := range r.Containers() {
				res, _ := c["resources"].(map[string]any)
				lim, _ := res["limits"].(map[string]any)
				if _, ok := lim["cpu"]; ok {
					out = append(out, fmt.Sprintf("container %q sets a cpu limit (throttling risk)", containerName(c)))
				}
			}
			return out
		},
	},
	{
		ID: "no-readiness-probe", Severity: Medium,
		Why: "Without a readiness probe, traffic is routed to pods before they're ready.",
		Fix: "add a readinessProbe to each serving container",
		Check: func(r model.Resource) []string {
			// Only meaningful for long-running serving workloads.
			switch r.Kind {
			case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet":
			default:
				return nil
			}
			var out []string
			for _, c := range r.Containers() {
				if _, ok := c["readinessProbe"]; !ok {
					out = append(out, fmt.Sprintf("container %q has no readiness probe", containerName(c)))
				}
			}
			return out
		},
	},
	{
		ID: "no-liveness-probe", Severity: Low,
		Why: "Without a liveness probe, a hung container is never restarted.",
		Fix: "add a livenessProbe where a hang is detectable",
		Check: func(r model.Resource) []string {
			switch r.Kind {
			case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet":
			default:
				return nil
			}
			var out []string
			for _, c := range r.Containers() {
				if _, ok := c["livenessProbe"]; !ok {
					out = append(out, fmt.Sprintf("container %q has no liveness probe", containerName(c)))
				}
			}
			return out
		},
	},
	{
		ID: "single-replica", Severity: Medium,
		Why: "A single-replica Deployment has no redundancy; a node drain or crash is downtime.",
		Fix: "set replicas >= 2 (and a PodDisruptionBudget)",
		Check: func(r model.Resource) []string {
			if r.Kind != "Deployment" {
				return nil
			}
			if v, ok := model.IntField(r.Get("spec", "replicas")); ok && v <= 1 {
				return []string{fmt.Sprintf("Deployment has replicas: %d (no redundancy)", v)}
			}
			return nil
		},
	},

	// ---- Security ----
	{
		ID: "runs-as-root", Severity: High,
		Why: "Container may run as root (runAsNonRoot not set at pod or container level).",
		Fix: "set securityContext.runAsNonRoot: true (and runAsUser)",
		Check: func(r model.Resource) []string {
			if !isWorkload(r) {
				return nil
			}
			podNonRoot, podSet := boolField(podSecurityContext(r), "runAsNonRoot")
			var out []string
			for _, c := range r.Containers() {
				csc, _ := c["securityContext"].(map[string]any)
				cNonRoot, cSet := boolField(csc, "runAsNonRoot")
				nonRoot := (cSet && cNonRoot) || (!cSet && podSet && podNonRoot)
				if !nonRoot {
					out = append(out, fmt.Sprintf("container %q may run as root", containerName(c)))
				}
			}
			return out
		},
	},
	{
		ID: "privileged", Severity: High,
		Why: "A privileged container has full host access; a compromise escapes the container.",
		Fix: "remove securityContext.privileged: true",
		Check: func(r model.Resource) []string {
			if !isWorkload(r) {
				return nil
			}
			var out []string
			for _, c := range r.Containers() {
				csc, _ := c["securityContext"].(map[string]any)
				if v, ok := boolField(csc, "privileged"); ok && v {
					out = append(out, fmt.Sprintf("container %q is privileged", containerName(c)))
				}
			}
			return out
		},
	},
	{
		ID: "allow-priv-escalation", Severity: Medium,
		Why: "allowPrivilegeEscalation is not disabled; a process can gain more privileges than its parent.",
		Fix: "set securityContext.allowPrivilegeEscalation: false",
		Check: func(r model.Resource) []string {
			if !isWorkload(r) {
				return nil
			}
			var out []string
			for _, c := range r.Containers() {
				csc, _ := c["securityContext"].(map[string]any)
				v, ok := boolField(csc, "allowPrivilegeEscalation")
				if !ok || v {
					out = append(out, fmt.Sprintf("container %q does not disable privilege escalation", containerName(c)))
				}
			}
			return out
		},
	},
	{
		ID: "host-namespace", Severity: High,
		Why: "Pod shares a host namespace (network/PID/IPC), weakening isolation.",
		Fix: "remove hostNetwork/hostPID/hostIPC unless truly required",
		Check: func(r model.Resource) []string {
			ps := r.PodTemplateSpec()
			if ps == nil {
				return nil
			}
			var out []string
			for _, key := range []string{"hostNetwork", "hostPID", "hostIPC"} {
				if v, ok := boolField(ps, key); ok && v {
					out = append(out, fmt.Sprintf("pod sets %s: true", key))
				}
			}
			return out
		},
	},
	{
		ID: "host-path-mount", Severity: High,
		Why: "A hostPath volume mounts the node filesystem into the pod.",
		Fix: "avoid hostPath; use a proper PVC or CSI volume",
		Check: func(r model.Resource) []string {
			ps := r.PodTemplateSpec()
			if ps == nil {
				return nil
			}
			var out []string
			for _, v := range toMapSlice(ps["volumes"]) {
				if _, ok := v["hostPath"]; ok {
					name, _ := v["name"].(string)
					out = append(out, fmt.Sprintf("volume %q uses hostPath", name))
				}
			}
			return out
		},
	},
	{
		ID: "added-capabilities", Severity: Medium,
		Why: "Container adds Linux capabilities beyond the default set.",
		Fix: "drop added capabilities; prefer capabilities.drop: [ALL]",
		Check: func(r model.Resource) []string {
			if !isWorkload(r) {
				return nil
			}
			var out []string
			for _, c := range r.Containers() {
				csc, _ := c["securityContext"].(map[string]any)
				caps, _ := csc["capabilities"].(map[string]any)
				if added := toStrSlice(caps["add"]); len(added) > 0 {
					out = append(out, fmt.Sprintf("container %q adds capabilities: %s", containerName(c), strings.Join(added, ",")))
				}
			}
			return out
		},
	},
	{
		ID: "secret-in-env", Severity: Medium,
		Why: "A literal value in env looks like a secret and will leak in specs, logs, and `describe`.",
		Fix: "reference secrets via valueFrom.secretKeyRef, not a literal value",
		Check: func(r model.Resource) []string {
			if !isWorkload(r) {
				return nil
			}
			var out []string
			for _, c := range r.Containers() {
				for _, e := range toMapSlice(c["env"]) {
					name, _ := e["name"].(string)
					val, hasVal := e["value"].(string)
					if hasVal && looksSecret(name) && val != "" {
						out = append(out, fmt.Sprintf("container %q sets env %q to a literal value (likely a secret)", containerName(c), name))
					}
				}
			}
			return out
		},
	},
	{
		ID: "writable-root-fs", Severity: Low,
		Why: "Root filesystem is writable; a compromised process can modify the image at runtime.",
		Fix: "set securityContext.readOnlyRootFilesystem: true",
		Check: func(r model.Resource) []string {
			if !isWorkload(r) {
				return nil
			}
			var out []string
			for _, c := range r.Containers() {
				csc, _ := c["securityContext"].(map[string]any)
				if v, ok := boolField(csc, "readOnlyRootFilesystem"); !ok || !v {
					out = append(out, fmt.Sprintf("container %q has a writable root filesystem", containerName(c)))
				}
			}
			return out
		},
	},
	{
		ID: "wildcard-rbac", Severity: High,
		Why: "An RBAC rule grants '*' verbs or resources — effectively cluster-wide power.",
		Fix: "scope rules to the specific verbs/resources needed",
		Check: func(r model.Resource) []string {
			if r.Kind != "Role" && r.Kind != "ClusterRole" {
				return nil
			}
			var out []string
			for _, rule := range toMapSlice(r.Get("rules")) {
				if hasWildcard(rule["verbs"]) || hasWildcard(rule["resources"]) || hasWildcard(rule["apiGroups"]) {
					out = append(out, fmt.Sprintf("%s grants wildcard (*) verbs/resources", r.Ref()))
					break
				}
			}
			return out
		},
	},

	// ---- Image / hygiene ----
	{
		ID: "image-latest-or-untagged", Severity: Medium,
		Why: "Using :latest (or no tag) makes deploys non-reproducible and rollbacks unreliable.",
		Fix: "pin an immutable tag or digest",
		Check: func(r model.Resource) []string {
			if !isWorkload(r) {
				return nil
			}
			var out []string
			for _, c := range r.Containers() {
				img, _ := c["image"].(string)
				if img == "" {
					continue
				}
				if isLatestOrUntagged(img) {
					out = append(out, fmt.Sprintf("container %q uses image %q (not pinned)", containerName(c), img))
				}
			}
			return out
		},
	},
	{
		ID: "pull-policy-always-on-pinned", Severity: Low,
		Why: "imagePullPolicy: Always on a pinned tag re-pulls every start and can surprise-swap a mutated tag.",
		Fix: "use IfNotPresent for pinned tags",
		Check: func(r model.Resource) []string {
			if !isWorkload(r) {
				return nil
			}
			var out []string
			for _, c := range r.Containers() {
				img, _ := c["image"].(string)
				policy, _ := c["imagePullPolicy"].(string)
				if policy == "Always" && img != "" && !isLatestOrUntagged(img) {
					out = append(out, fmt.Sprintf("container %q pins %q but pulls Always", containerName(c), img))
				}
			}
			return out
		},
	},
	{
		ID: "deprecated-api-version", Severity: High,
		Why: "This apiVersion is removed in current Kubernetes; the manifest will be rejected on apply/upgrade.",
		Fix: "migrate to the current apiVersion for this kind",
		Check: func(r model.Resource) []string {
			gv := r.APIVersion + "/" + r.Kind
			if replacement, ok := deprecatedAPIs[gv]; ok {
				return []string{fmt.Sprintf("%s is removed; use %s", r.APIVersion, replacement)}
			}
			return nil
		},
	},
	{
		ID: "hardcoded-namespace", Severity: Low,
		Why: "A hardcoded metadata.namespace prevents deploying the same manifest to multiple namespaces.",
		Fix: "omit namespace and set it at apply time (or via Kustomize/Helm)",
		Check: func(r model.Resource) []string {
			if r.Namespace != "" && r.Namespace != "default" {
				return []string{fmt.Sprintf("hardcodes namespace: %q", r.Namespace)}
			}
			return nil
		},
	},
	{
		ID: "automount-sa-token", Severity: Low,
		Why: "The default ServiceAccount token is auto-mounted; a compromised pod can call the API server.",
		Fix: "set automountServiceAccountToken: false unless the pod needs the API",
		Check: func(r model.Resource) []string {
			ps := r.PodTemplateSpec()
			if ps == nil {
				return nil
			}
			if v, ok := boolField(ps, "automountServiceAccountToken"); !ok || v {
				return []string{"ServiceAccount token is auto-mounted (default)"}
			}
			return nil
		},
	},
}

// deprecatedAPIs maps removed apiVersion/Kind to the replacement apiVersion.
// Small, high-signal set of the common ones; extend freely.
var deprecatedAPIs = map[string]string{
	"extensions/v1beta1/Ingress":                        "networking.k8s.io/v1",
	"networking.k8s.io/v1beta1/Ingress":                 "networking.k8s.io/v1",
	"extensions/v1beta1/Deployment":                     "apps/v1",
	"apps/v1beta1/Deployment":                           "apps/v1",
	"apps/v1beta2/Deployment":                           "apps/v1",
	"extensions/v1beta1/DaemonSet":                      "apps/v1",
	"extensions/v1beta1/ReplicaSet":                     "apps/v1",
	"policy/v1beta1/PodDisruptionBudget":                "policy/v1",
	"batch/v1beta1/CronJob":                             "batch/v1",
	"autoscaling/v2beta2/HorizontalPodAutoscaler":       "autoscaling/v2",
	"autoscaling/v2beta1/HorizontalPodAutoscaler":       "autoscaling/v2",
	"rbac.authorization.k8s.io/v1beta1/Role":            "rbac.authorization.k8s.io/v1",
	"rbac.authorization.k8s.io/v1beta1/ClusterRole":     "rbac.authorization.k8s.io/v1",
	"storage.k8s.io/v1beta1/CSIDriver":                  "storage.k8s.io/v1",
	"apiextensions.k8s.io/v1beta1/CustomResourceDefinition": "apiextensions.k8s.io/v1",
}

// ---- small helpers used only by rules ----

func toMapSlice(v any) []map[string]any {
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

func toStrSlice(v any) []string {
	s, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(s))
	for _, e := range s {
		if str, ok := e.(string); ok {
			out = append(out, str)
		}
	}
	return out
}

func hasWildcard(v any) bool {
	for _, s := range toStrSlice(v) {
		if s == "*" {
			return true
		}
	}
	return false
}

func looksSecret(name string) bool {
	n := strings.ToUpper(name)
	for _, kw := range []string{"PASSWORD", "PASSWD", "SECRET", "TOKEN", "APIKEY", "API_KEY", "PRIVATE_KEY", "ACCESS_KEY"} {
		if strings.Contains(n, kw) {
			return true
		}
	}
	return false
}

func isLatestOrUntagged(image string) bool {
	// Strip a digest first; a digest counts as pinned.
	if strings.Contains(image, "@sha256:") {
		return false
	}
	// Separate tag from the last path segment (avoid the registry port colon).
	slash := strings.LastIndex(image, "/")
	lastSeg := image[slash+1:]
	colon := strings.LastIndex(lastSeg, ":")
	if colon == -1 {
		return true // no tag
	}
	tag := lastSeg[colon+1:]
	return tag == "" || tag == "latest"
}
