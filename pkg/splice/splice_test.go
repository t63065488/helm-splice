package splice_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/t63065488/helm-splice/pkg/splice"
	"gopkg.in/yaml.v3"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdirall: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("writefile %s: %v", p, err)
	}
	return p
}

func unmarshalYAML(t *testing.T, data []byte) interface{} {
	t.Helper()
	var v interface{}
	if err := yaml.Unmarshal(data, &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return v
}

func TestBasicSplice(t *testing.T) {
	d := t.TempDir()
	writeFile(t, d, "service-values.yaml", "svc:\n  a: 1\n  b:\n    c: d\n")
	writeFile(t, d, "values.yaml", "service: service-values.yaml\n")

	out, err := splice.ResolveFileToYAML(filepath.Join(d, "values.yaml"), splice.Options{})
	if err != nil {
		t.Fatalf("ResolveFileToYAML: %v", err)
	}

	v := unmarshalYAML(t, out)
	m, ok := v.(map[string]interface{})
	if !ok {
		t.Fatalf("expected mapping at root, got %T", v)
	}
	service, ok := m["service"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected service to be mapping, got %T", m["service"])
	}
	// resolved document has top-level key "svc" per the test fixture
	svc, ok := service["svc"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected service.svc to be mapping, got %T", service["svc"])
	}
	if fmtSprint(svc["a"]) != "1" {
		t.Fatalf("unexpected service.svc.a: %#v", svc["a"])
	}
}

func TestEnvSubstitution(t *testing.T) {
	d := t.TempDir()
	writeFile(t, d, "service-values-prod.yaml", "deployment:\n  replicas: 3\n")
	writeFile(t, d, "values.yaml", "service: service-values-{{ env }}.yaml\n")

	out, err := splice.ResolveFileToYAML(filepath.Join(d, "values.yaml"), splice.Options{Env: "prod"})
	if err != nil {
		t.Fatalf("ResolveFileToYAML: %v", err)
	}

	v := unmarshalYAML(t, out)
	m := v.(map[string]interface{})
	svc := m["service"].(map[string]interface{})
	// resolved doc has top-level "deployment"
	deployment := svc["deployment"].(map[string]interface{})
	if fmtSprint(deployment["replicas"]) != "3" {
		t.Fatalf("expected replicas=3, got %#v", deployment["replicas"])
	}
}

func TestCircularReference(t *testing.T) {
	d := t.TempDir()
	writeFile(t, d, "a.yaml", "b: b.yaml\n")
	writeFile(t, d, "b.yaml", "a: a.yaml\n")

	_, err := splice.ResolveFileToYAML(filepath.Join(d, "a.yaml"), splice.Options{})
	if err == nil {
		t.Fatalf("expected circular reference error, got nil")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Fatalf("expected circular error, got: %v", err)
	}
}

func TestEmptyReferencedFile(t *testing.T) {
	d := t.TempDir()
	writeFile(t, d, "empty.yaml", "")
	writeFile(t, d, "values.yaml", "ref: empty.yaml\n")

	_, err := splice.ResolveFileToYAML(filepath.Join(d, "values.yaml"), splice.Options{})
	if err == nil {
		t.Fatalf("expected error for empty referenced file, got nil")
	}
	if !strings.Contains(err.Error(), "referenced file") {
		t.Fatalf("expected referenced file error, got: %v", err)
	}
}

func TestNonMappingTopLevel(t *testing.T) {
	d := t.TempDir()
	writeFile(t, d, "list.yaml", "- a\n- b\n")

	out, err := splice.ResolveFileToYAML(filepath.Join(d, "list.yaml"), splice.Options{})
	if err != nil {
		t.Fatalf("ResolveFileToYAML: %v", err)
	}
	v := unmarshalYAML(t, out)
	lst, ok := v.([]interface{})
	if !ok {
		t.Fatalf("expected top-level list, got %T", v)
	}
	if len(lst) != 2 {
		t.Fatalf("expected list length 2, got %d", len(lst))
	}
}

// fmtSprint produces a YAML-like string for simple values to facilitate assertions
func fmtSprint(v interface{}) string {
	// Use yaml.Marshal to produce a normalized textual representation
	b, _ := yaml.Marshal(v)
	return strings.TrimSpace(string(b))
}
