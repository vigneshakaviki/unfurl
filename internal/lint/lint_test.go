package lint

import (
	"testing"

	"github.com/vignesh/unfurl/internal/render"
)

// findByRule returns the findings emitted by a given rule ID.
func findByRule(fs []Finding, id string) []Finding {
	var out []Finding
	for _, f := range fs {
		if f.RuleID == id {
			out = append(out, f)
		}
	}
	return out
}

func mustParse(t *testing.T, yaml string) []Finding {
	t.Helper()
	res, err := render.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return Run(res)
}

func TestPrivilegedAndRoot(t *testing.T) {
	fs := mustParse(t, `
apiVersion: apps/v1
kind: Deployment
metadata: {name: web}
spec:
  replicas: 2
  template:
    spec:
      containers:
        - name: c
          image: nginx:1.27
          securityContext: {privileged: true}
`)
	if len(findByRule(fs, "privileged")) != 1 {
		t.Errorf("expected privileged finding, got %+v", fs)
	}
	if len(findByRule(fs, "runs-as-root")) != 1 {
		t.Errorf("expected runs-as-root finding")
	}
	// replicas: 2 must NOT trigger single-replica.
	if len(findByRule(fs, "single-replica")) != 0 {
		t.Errorf("did not expect single-replica for replicas:2")
	}
}

func TestNonRootSuppressesRule(t *testing.T) {
	fs := mustParse(t, `
apiVersion: apps/v1
kind: Deployment
metadata: {name: web}
spec:
  template:
    spec:
      securityContext: {runAsNonRoot: true}
      containers:
        - name: c
          image: nginx:1.27
`)
	if n := len(findByRule(fs, "runs-as-root")); n != 0 {
		t.Errorf("pod-level runAsNonRoot should suppress rule, got %d findings", n)
	}
}

func TestImagePinning(t *testing.T) {
	cases := map[string]bool{ // image -> should flag
		"nginx":                       true,
		"nginx:latest":                true,
		"nginx:1.27":                  false,
		"registry:5000/app:1.0":       false, // registry port must not fool the tag parser
		"registry:5000/app":           true,
		"app@sha256:abc123":           false,
	}
	for img, want := range cases {
		if got := isLatestOrUntagged(img); got != want {
			t.Errorf("isLatestOrUntagged(%q) = %v, want %v", img, got, want)
		}
	}
}

func TestDeprecatedAPI(t *testing.T) {
	fs := mustParse(t, `
apiVersion: extensions/v1beta1
kind: Ingress
metadata: {name: old}
`)
	if len(findByRule(fs, "deprecated-api-version")) != 1 {
		t.Errorf("expected deprecated-api-version finding")
	}
}

func TestSecretInEnv(t *testing.T) {
	fs := mustParse(t, `
apiVersion: apps/v1
kind: Deployment
metadata: {name: web}
spec:
  template:
    spec:
      containers:
        - name: c
          image: nginx:1.27
          env:
            - {name: DB_PASSWORD, value: hunter2}
            - {name: LOG_LEVEL, value: info}
`)
	got := findByRule(fs, "secret-in-env")
	if len(got) != 1 {
		t.Fatalf("expected 1 secret-in-env finding, got %d", len(got))
	}
}

func TestWildcardRBAC(t *testing.T) {
	fs := mustParse(t, `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata: {name: god}
rules:
  - apiGroups: ["*"]
    resources: ["*"]
    verbs: ["*"]
`)
	if len(findByRule(fs, "wildcard-rbac")) != 1 {
		t.Errorf("expected wildcard-rbac finding")
	}
}

func TestSeverityOrdering(t *testing.T) {
	fs := mustParse(t, `
apiVersion: apps/v1
kind: Deployment
metadata: {name: web, namespace: prod}
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: c
          image: nginx:latest
`)
	// First finding must be the highest severity present.
	if len(fs) == 0 || fs[0].Severity != High {
		t.Errorf("expected findings sorted high-first, got %+v", fs)
	}
}
