# Releasing

Releases are automated with [GoReleaser](https://goreleaser.com) via
`.github/workflows/release.yml`, triggered by pushing a `v*` tag.

## One-time setup

1. **Create the Homebrew tap repo** (public): `vigneshakaviki/homebrew-tap`.
2. **Create a PAT** with `contents:write` on that tap repo (fine-grained token,
   Repository access → `homebrew-tap`, Contents: Read and write).
3. **Add it as a secret** on `vigneshakaviki/unfurl`:
   Settings → Secrets and variables → Actions → `HOMEBREW_TAP_TOKEN`.

If you don't want a Homebrew tap yet, delete the `brews:` block from
`.goreleaser.yaml`; everything else (binaries, checksums, GitHub Release) still
works with the built-in `GITHUB_TOKEN`.

## Cut a release

```sh
git tag v0.1.0
git push origin v0.1.0
```

The workflow runs tests, then builds linux/darwin/windows × amd64/arm64,
publishes a GitHub Release with archives + checksums, and (if the tap token is
set) updates the Homebrew formula.

## Install after release

```sh
brew install vigneshakaviki/tap/unfurl          # via the tap
go install github.com/vigneshakaviki/unfurl@v0.1.0
# or download a prebuilt archive from the GitHub Release page
```

## Local dry run (optional)

```sh
goreleaser release --snapshot --clean   # builds into ./dist without publishing
```
