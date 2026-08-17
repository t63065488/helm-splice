# helm-splice

`helm-splice` is a Helm plugin and Go library that resolves and inlines external YAML values files inside a Helm values file. It replaces placeholder references—such as `[[ service-values.yaml ]]` or `[[ secrets://secret-values.yaml ]]`—with the full parsed contents of the referenced file.

It operates as a **Helm Downloader Plugin**, allowing you to pass custom `splice://` URIs directly to standard `helm` commands via `-f` / `--values`.

---

## Key Features

* **Inline Placeholders**: Resolves `[[ file.yaml ]]` references (supports both quoted `"[[ file.yaml ]]"` and unquoted flow sequence `[[ file.yaml ]]` syntax).
* **Native Helm Downloader Protocol**: Use `helm install -f splice://values.yaml` natively without process substitution or temporary files.
* **Encrypted Secrets Support**: Decrypts encrypted files on the fly when referenced with the `secrets://` protocol (delegating to `helm secrets` / `sops`).
* **Environment Parameterization**: Dynamically substitutes `{{ env }}` tokens inside reference paths (e.g., `[[ values-{{ env }}.yaml ]]`).
* **Cycle Detection**: Safely resolves nested placeholders while preventing circular reference loops.

---

## Key Pieces

* **Library implementing the splice logic**: [`pkg/splice`](https://www.google.com/search?q=pkg/splice)
* **CLI entrypoint**: [`cmd/helm-splice`](https://www.google.com/search?q=cmd/helm-splice/main.go)
* **Helm plugin manifest**: [`plugin.yaml`](https://www.google.com/search?q=plugin.yaml)
* **Install hook**: [`scripts/install-binary.sh`](https://www.google.com/search?q=scripts/install-binary.sh)

---

## Placeholder Syntax & Example

Inside a parent values file, specify references under top-level mapping keys:

### 1. `values.yaml` (Umbrella Chart)

```yaml
appName: "my-app"

# Standard file reference (quoted or unquoted flow sequence)
service: "[[ service-values.yaml ]]"

# Encrypted secret file reference
database: "[[ secrets://db-credentials.yaml ]]"

# Environment-parameterized file reference
config: "[[ config-{{ env }}.yaml ]]"

```

### 2. Referenced Files

`service-values.yaml`

```yaml
svc:
  port: 8080
  replicas: 3

```

`db-credentials.yaml` *(SOPS / Vault Encrypted File)*

```yaml
sops: ...
host: "db.internal"
password: "ENC[AES256_GCM,data:...]"

```

### 3. Spliced Output

When evaluated, placeholders are replaced with the parsed mapping content of the target files:

```yaml
appName: "my-app"
service:
  svc:
    port: 8080
    replicas: 3
database:
  host: "db.internal"
  password: "supersecretpassword"
config:
  ...

```

---

## Installation

Install the plugin directly from GitHub using Helm's plugin manager:

```bash
helm plugin install https://github.com/t63065488/helm-splice

```

Or install a specific tag or branch:

```bash
helm plugin install https://github.com/t63065488/helm-splice.git#<ref>

```

Verify the installation:

```bash
helm plugin list

```

---

## Usage with Helm

Because `helm-splice` registers as a Helm Downloader Plugin for the `splice://` scheme, you can pass URLs directly to standard Helm commands:

### Basic Splicing

```bash
helm template my-release ./chart -f splice://values.yaml
helm install my-release ./chart -f splice://values.yaml

```

### Passing Environment Parameters (`?env=...`)

To resolve references containing `{{ env }}` tokens (e.g., `[[ service-{{ env }}.yaml ]]`), supply the `env` value as a URL query parameter:

```bash
helm template my-release ./chart -f "splice://values.yaml?env=prod"

```

### Splicing Encrypted Files (`secrets://`)

When a placeholder specifies `secrets://` (e.g. `[[ secrets://db-credentials.yaml ]]`), `helm-splice` automatically invokes `helm secrets view` (or `sops`) to decrypt the file in memory prior to splicing:

```bash
helm template my-release ./chart -f splice://values.yaml

```

---

## CLI Usage

You can also execute the `helm-splice` binary directly:

```bash
# Build locally
go build -o ./bin/helm-splice ./cmd/helm-splice

# Standard file resolution
./bin/helm-splice values.yaml

# With --env flag
./bin/helm-splice --env prod values.yaml

# Processing splice:// URIs directly
./bin/helm-splice "splice://values.yaml?env=prod"

```

---

## Testing

Run the test suite using standard Go test commands:

```bash
go test ./pkg/splice -v

```

---

## Contributing

Contributions are welcome! Suggested areas for contribution:

* Additional AST parsing edge cases
* Expanded error reporting and validation tests

---

## License

See the repository [LICENSE](https://www.google.com/search?q=LICENSE).