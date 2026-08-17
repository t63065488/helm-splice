// Package splice implements resolution of "[[ file.yaml ]]" references
// found as the value of a top-level key in a Helm values file. Each
// reference is replaced with the full parsed contents of the referenced
// YAML file, enabling a chart's own environment-specific values file to
// be reused unmodified inside an umbrella chart's values file.
package splice

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// placeholderPattern matches the reference path inside a "[[ ... ]]"
// token, e.g. "../service-a/values-{{ env }}.yaml".
//
// Note: "[[ foo.yaml ]]" is valid YAML flow syntax for a sequence
// nested inside another sequence (list-of-list), NOT a plain scalar
// string. yaml.v3 parses it as SequenceNode -> SequenceNode ->
// ScalarNode. extractPlaceholder below detects that specific shape
// rather than matching against a scalar's raw text.
var placeholderPattern = regexp.MustCompile(`^\s*(.+?)\s*$`)

// envTokenPattern matches the {{ env }} token inside a reference path,
// tolerating varying internal whitespace ("{{env}}", "{{ env }}").
var envTokenPattern = regexp.MustCompile(`\{\{\s*env\s*\}\}`)

// Options controls how references are resolved.
type Options struct {
	// Env substitutes into {{ env }} tokens inside reference paths.
	// May be empty if no reference in the file tree uses the token.
	Env string
}

// ResolveFile loads the values file at path, resolves every top-level
// "[[ file.yaml ]]" reference (recursively, with cycle detection), and
// returns the fully-spliced document as a *yaml.Node ready to be
// re-marshaled.
func ResolveFile(path string, opts Options) (*yaml.Node, error) {
	visiting := map[string]bool{}
	return resolveFile(path, opts, visiting)
}

// ResolveFileToYAML is a convenience wrapper that resolves path and
// marshals the result back to YAML bytes.
func ResolveFileToYAML(path string, opts Options) ([]byte, error) {
	node, err := ResolveFile(path, opts)
	if err != nil {
		return nil, err
	}
	return yaml.Marshal(node)
}

func resolveFile(path string, opts Options, visiting map[string]bool) (*yaml.Node, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving path %q: %w", path, err)
	}

	if visiting[absPath] {
		return nil, fmt.Errorf("circular splice reference detected at %q", absPath)
	}
	visiting[absPath] = true
	defer delete(visiting, absPath)

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", absPath, err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing %q: %w", absPath, err)
	}

	// An empty file unmarshal into a zero-value node; nothing to splice.
	if len(doc.Content) == 0 {
		return &doc, nil
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		// Not a mapping at the top level (e.g. a bare scalar or list).
		// Nothing to splice into; return as-is.
		return &doc, nil
	}

	for i := 0; i < len(root.Content); i += 2 {
		keyNode := root.Content[i]
		valNode := root.Content[i+1]

		if valNode.Kind != yaml.ScalarNode {
			continue
		}

		match := placeholderPattern.FindStringSubmatch(valNode.Value)
		if match == nil {
			continue
		}

		refTemplate := match[1]
		refPath, err := substituteEnv(refTemplate, opts.Env)
		if err != nil {
			return nil, fmt.Errorf("key %q in %q: %w", keyNode.Value, absPath, err)
		}

		resolvedPath := refPath
		if !filepath.IsAbs(resolvedPath) {
			resolvedPath = filepath.Join(filepath.Dir(absPath), refPath)
		}

		resolvedDoc, err := resolveFile(resolvedPath, opts, visiting)
		if err != nil {
			return nil, fmt.Errorf("splicing key %q in %q: %w", keyNode.Value, absPath, err)
		}
		if len(resolvedDoc.Content) == 0 {
			return nil, fmt.Errorf("splicing key %q in %q: referenced file %q is empty", keyNode.Value, absPath, resolvedPath)
		}

		// Replace the placeholder scalar node with the resolved
		// document's root node (typically a mapping).
		*valNode = *resolvedDoc.Content[0]
	}

	return &doc, nil
}

func substituteEnv(refTemplate, env string) (string, error) {
	if !strings.Contains(refTemplate, "{{") {
		return refTemplate, nil
	}
	if env == "" {
		return "", fmt.Errorf("reference %q requires --env but none was provided", refTemplate)
	}
	return envTokenPattern.ReplaceAllString(refTemplate, env), nil
}
