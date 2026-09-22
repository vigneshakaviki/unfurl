package lint

import (
	"fmt"
	"strings"
	"testing"
)

func TestProbeRulesDistinguishInitContainersAndSidecars(t *testing.T) {
	for _, kind := range []string{"Deployment", "StatefulSet", "DaemonSet", "ReplicaSet"} {
		t.Run(kind, func(t *testing.T) {
			fs := mustParse(t, fmt.Sprintf(`
apiVersion: apps/v1
kind: %s
metadata: {name: app}
spec:
  template:
    spec:
      initContainers:
        - name: migrate
          image: migrate:1.0
          securityContext: {privileged: true}
        - name: proxy
          image: proxy:1.0
          restartPolicy: Always
        - name: healthy-sidecar
          image: proxy:1.0
          restartPolicy: Always
          readinessProbe: {tcpSocket: {port: 8080}}
          livenessProbe: {tcpSocket: {port: 8080}}
      containers:
        - name: app
          image: app:1.0
        - name: healthy-app
          image: app:1.0
          readinessProbe: {tcpSocket: {port: 8080}}
          livenessProbe: {tcpSocket: {port: 8080}}
`, kind))
			for _, rule := range []string{"no-readiness-probe", "no-liveness-probe"} {
				got := findByRule(fs, rule)
				if len(got) != 2 {
					t.Fatalf("%s: want 2 findings for app and proxy, got %+v", rule, got)
				}
				for i, name := range []string{"proxy", "app"} {
					if !strings.Contains(got[i].Message, fmt.Sprintf("container %q", name)) {
						t.Errorf("%s: unexpected finding: %+v", rule, got[i])
					}
				}
			}
			if got := findByRule(fs, "privileged"); len(got) != 1 || !strings.Contains(got[0].Message, "migrate") {
				t.Errorf("init container security check lost: %+v", got)
			}
			if got := findByRule(fs, "no-resource-limits"); len(got) != 5 {
				t.Errorf("resource checks must cover all 5 containers, got %+v", got)
			}
		})
	}
}

func TestProbeRulesKeepBatchWorkloadsExcluded(t *testing.T) {
	for _, kind := range []string{"Job", "CronJob", "Pod", "ConfigMap"} {
		fs := mustParse(t, fmt.Sprintf(`
apiVersion: v1
kind: %s
metadata: {name: batch}
spec:
  containers:
    - {name: task, image: task:1.0}
  template:
    spec:
      containers:
        - {name: task, image: task:1.0}
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - {name: task, image: task:1.0}
`, kind))
		for _, rule := range []string{"no-readiness-probe", "no-liveness-probe"} {
			if got := findByRule(fs, rule); len(got) != 0 {
				t.Errorf("%s: unexpected %s findings: %+v", kind, rule, got)
			}
		}
	}
}
