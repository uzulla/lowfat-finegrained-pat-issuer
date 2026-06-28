# getfgpat

`lowfat-finegrained-pat-issuer` — a small CLI that helps you create a GitHub
**Fine-grained Personal Access Token (PAT) for use with the [`gh`](https://cli.github.com/) CLI**.

It builds a GitHub PAT-creation URL with the permissions `gh` commonly needs
(plus a dated token name and description) already filled in, then opens it in
your browser. You review and click **Generate token**.

## Why

Creating a fine-grained PAT by hand means setting the owner, expiration, and a
handful of permissions every time. `getfgpat` pre-fills all of that from the
repositories you pass on the command line.

## Requirements

- [`gh`](https://cli.github.com/) is **optional**. When it is installed **and**
  authenticated (`gh auth login`), it is used to verify each repository and
  fetch its ID. If `gh` is missing or not logged in, validation is simply
  skipped — you pick the repositories on GitHub anyway.
- Go 1.21+ to build from source.

## Layout

```
.
├── src/          # source code
├── build/        # compiled binaries (created by make)
├── Makefile
└── go.mod
```

## Install

```sh
make build                      # builds ./build/getfgpat for your machine
mv build/getfgpat /usr/local/bin/   # or anywhere on your PATH
```

Or install into `$GOBIN` / `$GOPATH/bin`:

```sh
make install
```

Pre-built release binaries (Linux x64, macOS Apple Silicon):

```sh
make build-all          # outputs to ./build/
#   build/getfgpat-linux-amd64
#   build/getfgpat-darwin-arm64
```

## Usage

```sh
getfgpat owner/repo-a owner/repo-b      # pass repos as arguments
getfgpat                                # or enter them interactively
getfgpat --expires 30 owner/repo        # set expiration (days)
getfgpat --no-open owner/repo           # print the URL only
```

Options:

| Flag | Description |
|------|-------------|
| `--expires <days\|none>` | Token expiration, 1–366 days or `none`. Default: `90`. |
| `--no-open` | Print the URL only; don't launch a browser. |
| `-h`, `--help` | Show help. |

### Example

```
$ getfgpat uzulla/PPQB
  ✓ uzulla/PPQB                              id=968181478

Generated URL:
https://github.com/settings/personal-access-tokens/new?actions=read&contents=write&...

⚠ GitHub cannot pre-select repositories via URL.
  Under 'Repository access', choose 'Only select repositories' and add:
   - uzulla/PPQB
```

### What gets pre-filled

- **Name** — `<date> <repo short names>` (e.g. `2026-06-26 PPQB`), capped at 40 chars.
- **Description** — `PAT for the gh CLI (generated <date>). Repos: <owner/repo …>`.
- **Resource owner** (`target_name`) — the owner of the repositories.
- **Expiration** (`expires_in`).
- **Permissions** commonly used by `gh`:

  | Permission | Level |
  |------------|-------|
  | `contents` | write |
  | `metadata` | read |
  | `issues` | write |
  | `pull_requests` | write |
  | `actions` | read |

## Known limitation: repositories can't be auto-selected

GitHub's PAT page only supports pre-filling a fixed set of fields
(name, description, owner, expiration, permissions). The page is a client-side
React form, and **no URL parameter is wired to the repository picker** — so the
"Only select repositories" choice and the repo list cannot be set from the URL.

`getfgpat` works around this by:

1. embedding the repository list into the token **name** and **description**, and
2. printing the exact repositories for you to add manually after the page opens.

(The `repository_ids` query parameter is still included as best-effort, in case
GitHub ever wires it up, but it is currently ignored.)

Reference: [Managing your personal access tokens — GitHub Docs](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)

## Develop

```sh
make test     # run tests
make vet      # go vet
make fmt      # go fmt
make clean    # remove build artifacts
```

## Release

Pushing a version tag (`v*`) triggers the
[`release`](.github/workflows/release.yml) GitHub Actions workflow, which builds
the Linux x64 and macOS arm64 binaries and attaches them to a GitHub Release.

```sh
git tag v0.1.0
git push origin v0.1.0
# → release created with build/getfgpat-linux-amd64 and build/getfgpat-darwin-arm64 attached
```
