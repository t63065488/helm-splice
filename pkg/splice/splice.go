// Package splice implements resolution of "[[ file.yaml ]]" references
// found as the value of a top-level key in a Helm values file. Each
// reference is replaced with the full parsed contents of the referenced
// YAML file, enabling a chart's own environment-specific values file to
// be reused unmodified inside an umbrella chart's values file.
package splice

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	// envTokenPattern matches the {{ env }} token inside a reference path.
	envTokenPattern = regexp.MustCompile(`\{\{\s*env\s*\}\}`)

	// quotedPlaceholderPattern matches explicit string scalars like "[[ foo.yaml ]]".
	quotedPlaceholderPattern = regexp.MustCompile(`^\[\[\s*(.+?)\s*\]\]$`)
)

// Options controls how references are resolved.
type Options struct {
	// Env substitutes into {{ env }} tokens inside reference paths.
	// May be empty if no reference in the file tree uses the token.
	Env string
}

// CleanURI strips the "splice://" scheme and extracts path and query parameters (e.g. ?env=...).
func CleanURI(rawURI string) (path string, env string) {
	cleaned := strings.TrimPrefix(rawURI, "splice://")

	// Parse query parameters if present (e.g., splice://values.yaml?env=dev)
	if idx := strings.Index(cleaned, "?"); idx != -1 {
		pathPart := cleaned[:idx]
		queryPart := cleaned[idx+1:]
		if q, err := url.ParseQuery(queryPart); err == nil {
			return pathPart, q.Get("env")
		}
		return pathPart, ""
	}

	return cleaned, ""
}

// ResolveFile loads the values file at path, resolves every top-level
// "[[ file.yaml ]]" reference (recursively, with cycle detection), and
// returns the fully-spliced document as a *yaml.Node ready to be
// re-marshaled.
func ResolveFile(path string, opts Options) (*yaml.Node, error) {
	visiting := map[string]bool{}
	cleanPath, isSecret := parseProtocol(path)
	return resolveFile(cleanPath, isSecret, opts, visiting)
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

func resolveFile(path string, decrypt bool, opts Options, visiting map[string]bool) (*yaml.Node, error) {
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

	// Decrypt explicitly if the 'secrets://' protocol was specified for this file
	if decrypt {
		decrypted, err := decryptWithHelmSecrets(absPath)
		if err != nil {
			return nil, fmt.Errorf("decrypting %q: %w", absPath, err)
		}
		data = decrypted
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing %q: %w", absPath, err)
	}

	if len(doc.Content) == 0 {
		return &doc, nil
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return &doc, nil
	}

	for i := 0; i < len(root.Content); i += 2 {
		keyNode := root.Content[i]
		valNode := root.Content[i+1]

		refTemplate, isPlaceholder := extractPlaceholder(valNode)
		if !isPlaceholder {
			continue
		}

		refPath, err := substituteEnv(refTemplate, opts.Env)
		if err != nil {
			return nil, fmt.Errorf("key %q in %q: %w", keyNode.Value, absPath, err)
		}

		// Check for secrets:// or file:// protocol inside the placeholder
		cleanRef, isSecretRef := parseProtocol(refPath)

		resolvedPath := cleanRef
		if !filepath.IsAbs(resolvedPath) {
			resolvedPath = filepath.Join(filepath.Dir(absPath), cleanRef)
		}

		resolvedDoc, err := resolveFile(resolvedPath, isSecretRef, opts, visiting)
		if err != nil {
			return nil, fmt.Errorf("splicing key %q in %q: %w", keyNode.Value, absPath, err)
		}
		if len(resolvedDoc.Content) == 0 {
			return nil, fmt.Errorf("splicing key %q in %q: referenced file %q is empty", keyNode.Value, absPath, resolvedPath)
		}

		// Replace the placeholder node with the resolved document's root mapping node
		*valNode = *resolvedDoc.Content[0]
	}

	return &doc, nil
}

// decryptWithHelmSecrets delegates decryption to 'helm secrets view' with a fallback to direct 'sops'.
func decryptWithHelmSecrets(path string) ([]byte, error) {
	// 1. Try delegating to 'helm secrets view'
	if helmBin, err := exec.LookPath("helm"); err == nil {
		cmd := exec.Command(helmBin, "secrets", "view", path)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return out, nil
		}
	}

	// 2. Direct CLI fallback if 'helm secrets' plugin is absent or fails
	if sopsBin, err := exec.LookPath("sops"); err == nil {
		cmd := exec.Command(sopsBin, "--decrypt", "--output-type", "yaml", path)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return out, nil
		}
	}

	return nil, fmt.Errorf("unable to decrypt: ensure 'helm secrets' or 'sops' is available in PATH")
}

// parseProtocol checks if a path starts with secrets://, file://, or splice://,
// returning the clean path and a boolean indicating if it requires decryption.
func parseProtocol(ref string) (cleanPath string, isSecret bool) {
	if strings.HasPrefix(ref, "secrets://") {
		return strings.TrimPrefix(ref, "secrets://"), true
	}
	if strings.HasPrefix(ref, "file://") {
		return strings.TrimPrefix(ref, "file://"), false
	}
	if strings.HasPrefix(ref, "splice://") {
		return strings.TrimPrefix(ref, "splice://"), false
	}
	return ref, false
}

// extractPlaceholder detects both YAML flow sequences (`[[ file.yaml ]]`) and quoted scalars (`"[[ file.yaml ]]"`).
func extractPlaceholder(node *yaml.Node) (string, bool) {
	if node == nil {
		return "", false
	}

	// Unquoted flow sequence: [[ foo.yaml ]] -> SequenceNode -> SequenceNode -> ScalarNode
	if node.Kind == yaml.SequenceNode && len(node.Content) == 1 {
		innerSeq := node.Content[0]
		if innerSeq.Kind == yaml.SequenceNode && len(innerSeq.Content) == 1 {
			scalar := innerSeq.Content[0]
			if scalar.Kind == yaml.ScalarNode {
				return strings.TrimSpace(scalar.Value), true
			}
		}
	}

	// Quoted string scalar: "[[ foo.yaml ]]"
	if node.Kind == yaml.ScalarNode {
		if match := quotedPlaceholderPattern.FindStringSubmatch(node.Value); match != nil {
			return match[1], true
		}
	}

	return "", false
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
