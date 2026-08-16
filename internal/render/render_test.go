package render

import "testing"

func TestParseMultiDoc(t *testing.T) {
	in := `
apiVersion: v1
kind: ConfigMap
metadata: {name: a}
---
apiVersion: v1
kind: Service
metadata: {name: b, namespace: web}
`
	res, err := Parse([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 {
		t.Fatalf("want 2 resources, got %d", len(res))
	}
	if res[0].Kind != "ConfigMap" || res[0].Name != "a" {
		t.Errorf("bad first resource: %+v", res[0])
	}
	if res[1].Namespace != "web" {
		t.Errorf("want namespace web, got %q", res[1].Namespace)
	}
}

func TestParseSkipsEmptyAndComments(t *testing.T) {
	in := "# just a comment\n---\n\n---\napiVersion: v1\nkind: Pod\nmetadata: {name: p}\n"
	res, err := Parse([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Fatalf("want 1 resource, got %d", len(res))
	}
}

func TestParseFlattensList(t *testing.T) {
	in := `
apiVersion: v1
kind: List
items:
  - apiVersion: v1
    kind: Pod
    metadata: {name: p1}
  - apiVersion: v1
    kind: Pod
    metadata: {name: p2}
`
	res, err := Parse([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 {
		t.Fatalf("want 2 flattened resources, got %d", len(res))
	}
}

func TestContainersHelper(t *testing.T) {
	in := `
apiVersion: apps/v1
kind: Deployment
metadata: {name: d}
spec:
  template:
    spec:
      initContainers:
        - {name: init, image: busybox}
      containers:
        - {name: main, image: nginx}
`
	res, _ := Parse([]byte(in))
	cs := res[0].Containers()
	if len(cs) != 2 {
		t.Fatalf("want 2 containers (init+main), got %d", len(cs))
	}
}
