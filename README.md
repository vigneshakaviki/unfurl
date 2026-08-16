<div align="center">

# 🌱 unfurl

### See what your Helm chart *actually* deploys.

`unfurl` renders any Helm chart, Kustomize overlay, or pile of YAML into a
**readable summary** of what hits your cluster — then flags the **footguns**
before they do.

No AI required. No cluster required. Read-only. One static binary.

<!-- Hero GIF: run `vhs docs/demo.tape` to generate docs/demo.gif, then swap it in here. -->

```console
$ unfurl ./charts/api

📦 DEPLOYS
  Deployment/api     2 replicas · :8080 · ghcr.io/acme/api:1.4.2
  Service/api        ClusterIP → 80
  Ingress/api        api.acme.com

⚠️  FOOTGUNS  (2 high · 1 medium)
  🔴 high    Deployment/api   no resource limits   → set resources.limits
  🔴 high    Deployment/api   may run as root      → runAsNonRoot: true
  🟡 medium  Deployment/api   no readiness probe   → add readinessProbe
```

</div>

---

## Why

Kubernetes manifests are structured API objects, but we author them through
templated, whitespace-sensitive YAML. So the thing you actually ship is
*invisible* until it's live — and half the time it quietly runs as root, with no
limits, on a single replica, pulling `:latest`.

`unfurl` closes that gap. Point it at a chart and you see, in one screen:

- **📦 DEPLOYS** — every resource and what it does (replicas, ports, image, hosts)
- **⚠️ FOOTGUNS** — ~20 static checks for security, reliability, and upgrade landmines
- **🧠 PLAIN ENGLISH** — an optional, deterministic breakdown (`--explain`, still no AI)

## Install

```sh
# Homebrew
brew install vigneshakaviki/tap/unfurl

# Go
go install github.com/vigneshakaviki/unfurl@latest

# As a kubectl plugin — symlink onto PATH as `kubectl-unfurl`, then:
kubectl unfurl ./charts/api
```

## Usage

```sh
unfurl ./charts/api                       # Helm chart      -> helm template
unfurl ./overlays/prod                    # Kustomize dir   -> kustomize build
unfurl manifest.yaml                      # rendered YAML
helm template r ./chart | unfurl -        # stdin
```

Input type is auto-detected. Helm charts need `helm` on PATH; Kustomize dirs use
`kustomize` or fall back to `kubectl kustomize`. Raw YAML needs nothing.

### Example

```console
$ unfurl testdata/demo/api.yaml --explain

📦 DEPLOYS
  Deployment/api         1 replicas · :8080 · ghcr.io/acme/api:latest
  Ingress/api            api.acme.com
  Secret/api-db          2 keys
  Service/api            ClusterIP → 80

⚠️  FOOTGUNS  (5 high · 6 medium · 7 low)
  🔴 high    Deployment/api    container "api" is privileged          → remove securityContext.privileged: true
  🔴 high    Deployment/api    container "api" may run as root        → set securityContext.runAsNonRoot: true
  🔴 high    Deployment/api    container "api" has no resource limits → set resources.limits
  🔴 high    Ingress/legacy    extensions/v1beta1 is removed          → use networking.k8s.io/v1
  🟡 medium  Deployment/api    env "DB_PASSWORD" is a literal value   → use valueFrom.secretKeyRef
  ...

🧠 PLAIN ENGLISH
  • Deployment/api runs 1 copy of ghcr.io/acme/api:latest, listening on :8080.
    No resource limits — it can starve the node under load.
  • Ingress/api publishes the service to the outside world at api.acme.com.
```

### Flags

| Flag | Effect |
|------|--------|
| `-f <file>` | values file for Helm (repeatable) |
| `--release <name>` | Helm release name (default `unfurl`) |
| `--explain` | add the plain-English panel |
| `--lint` | footguns only, skip the DEPLOYS panel |
| `--json` | machine-readable output |
| `--fail-on <high\|medium\|low>` | exit `1` if any finding meets the threshold |
| `--no-color` | disable ANSI color (also respects `NO_COLOR`) |

### In CI

```yaml
- name: Lint rendered manifests
  run: helm template r ./charts/api | unfurl - --fail-on high
```

## The footgun checks

Security, reliability, and upgrade-safety rules, e.g.: runs-as-root, privileged,
host namespaces, hostPath mounts, added capabilities, secrets as literal env
values, wildcard RBAC, missing resource limits/requests, missing readiness/liveness
probes, single-replica Deployments, `:latest`/untagged images, and **removed API
versions** (so an upgrade doesn't reject your manifests).

See [`internal/lint/rules.go`](internal/lint/rules.go) for the full set — each rule
is a self-contained value, so [adding one](CONTRIBUTING.md) is a tiny PR.

## What unfurl is *not*

- Not a Helm replacement — it renders *through* Helm/Kustomize, changing nothing.
- Not a cluster scanner — it inspects manifests statically, before apply.
- Not a policy engine — for admission-time enforcement use Kyverno/OPA. `unfurl`
  is the fast, local "wait, what does this even do?" check.

## Contributing

New footgun rules and resource-summary improvements are the most welcome PRs.
Rules live as plain data in one file — see [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE)
