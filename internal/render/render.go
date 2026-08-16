// Package render turns an input (Helm chart, Kustomize dir, raw manifest, or
// stdin) into a flat list of parsed resources. It never re-implements Helm or
// Kustomize; it shells out to the real tools so output matches production.
package render

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vigneshakaviki/unfurl/internal/model"
	"gopkg.in/yaml.v3"
)

// Options control how a source is rendered.
type Options struct {
	ValuesFiles []string // -f, Helm only
	Release     string   // release name for `helm template`
}

// Source describes how the input was rendered, shown in the report header.
type Source struct {
	Kind string // "helm", "kustomize", "manifest", "stdin"
	Path string
}

// Render resolves path into rendered YAML and parses it into resources.
func Render(path string, opts Options) ([]model.Resource, Source, error) {
	raw, src, err := renderRaw(path, opts)
	if err != nil {
		return nil, src, err
	}
	res, err := Parse(raw)
	return res, src, err
}

func renderRaw(path string, opts Options) ([]byte, Source, error) {
	if path == "-" || path == "" {
		b, err := io.ReadAll(os.Stdin)
		return b, Source{Kind: "stdin", Path: "-"}, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, Source{}, fmt.Errorf("cannot read %q: %w", path, err)
	}

	if !info.IsDir() {
		b, err := os.ReadFile(path)
		return b, Source{Kind: "manifest", Path: path}, err
	}

	switch {
	case fileExists(filepath.Join(path, "Chart.yaml")):
		b, err := helmTemplate(path, opts)
		return b, Source{Kind: "helm", Path: path}, err
	case fileExists(filepath.Join(path, "kustomization.yaml")) ||
		fileExists(filepath.Join(path, "kustomization.yml")) ||
		fileExists(filepath.Join(path, "Kustomization")):
		b, err := kustomizeBuild(path)
		return b, Source{Kind: "kustomize", Path: path}, err
	default:
		b, err := concatYAMLDir(path)
		return b, Source{Kind: "manifest", Path: path}, err
	}
}

func helmTemplate(dir string, opts Options) ([]byte, error) {
	if _, err := exec.LookPath("helm"); err != nil {
		return nil, fmt.Errorf("this is a Helm chart but `helm` is not installed; install Helm or pass rendered YAML instead")
	}
	release := opts.Release
	if release == "" {
		release = "unfurl"
	}
	args := []string{"template", release, dir}
	for _, vf := range opts.ValuesFiles {
		args = append(args, "-f", vf)
	}
	return runTool("helm", args...)
}

func kustomizeBuild(dir string) ([]byte, error) {
	if _, err := exec.LookPath("kustomize"); err == nil {
		return runTool("kustomize", "build", dir)
	}
	if _, err := exec.LookPath("kubectl"); err == nil {
		return runTool("kubectl", "kustomize", dir)
	}
	return nil, fmt.Errorf("this is a Kustomize dir but neither `kustomize` nor `kubectl` is installed")
}

func concatYAMLDir(dir string) ([]byte, error) {
	var buf bytes.Buffer
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		buf.Write(b)
		buf.WriteString("\n---\n")
	}
	if buf.Len() == 0 {
		return nil, fmt.Errorf("no .yaml/.yml manifests found in %q", dir)
	}
	return buf.Bytes(), nil
}

func runTool(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s failed: %s", name, msg)
	}
	return stdout.Bytes(), nil
}

// Parse splits a multi-document YAML stream into resources, skipping empty and
// non-object documents (e.g. stray List wrappers are flattened).
func Parse(raw []byte) ([]model.Resource, error) {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	var out []model.Resource
	for {
		var doc any
		err := dec.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			return out, fmt.Errorf("parse error: %w", err)
		}
		m, ok := normalize(doc).(map[string]any)
		if !ok || len(m) == 0 {
			continue
		}
		// Flatten a List kind into its items.
		if k, _ := m["kind"].(string); k == "List" {
			for _, item := range toAnySlice(m["items"]) {
				if im, ok := item.(map[string]any); ok {
					out = append(out, fromMap(im))
				}
			}
			continue
		}
		out = append(out, fromMap(m))
	}
	return out, nil
}

func fromMap(m map[string]any) model.Resource {
	meta, _ := m["metadata"].(map[string]any)
	name, _ := meta["name"].(string)
	ns, _ := meta["namespace"].(string)
	kind, _ := m["kind"].(string)
	api, _ := m["apiVersion"].(string)
	return model.Resource{APIVersion: api, Kind: kind, Name: name, Namespace: ns, Raw: m}
}

// normalize converts map[interface{}]interface{} (from some YAML paths) into
// map[string]any recursively so downstream code can assert cleanly.
func normalize(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			t[k] = normalize(val)
		}
		return t
	case map[any]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[fmt.Sprint(k)] = normalize(val)
		}
		return m
	case []any:
		for i, val := range t {
			t[i] = normalize(val)
		}
		return t
	}
	return v
}

func toAnySlice(v any) []any {
	s, _ := v.([]any)
	return s
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
