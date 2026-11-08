# Agent instructions

## CI is mandatory

After every change — new plugin, new version, bug fix, or any code edit — run the full CI pipeline locally and fix all failures before pushing:

```bash
cd plugins/<name> && gofmt -l . && go vet ./... && go test -race ./... && go build ./... && go run . manifest >/dev/null
```

If CI fails, fix the root cause. Do not push code that breaks CI. The GitHub Actions workflow runs on every push and PR; a red build blocks releases.

## Repository structure

- Each plugin is its own Go module under `plugins/<name>/`
- `repository.json` is the catalog consumed by Silo servers
- CI matrix in `.github/workflows/ci.yml` must list every plugin
- Release workflow in `.github/workflows/release.yml` must include every plugin in: prefix mapping, expected assets, and manifest check

## Adding a new plugin

1. Create `plugins/<name>/` with `go.mod`, `manifest.json`, `main.go`, `Makefile`, `AGENTS.md`
2. Add to CI matrix in `.github/workflows/ci.yml`
3. Add to release workflow: prefix mapping, expected assets list, manifest check
4. Add entry to `repository.json`
5. Update `README.md` plugin table and build commands
6. Run full CI locally, fix all failures, then push

## Adding a new version

1. Bump version in `manifest.json`
2. Update any version references in README
3. Run full CI locally, fix all failures, then push
4. Tag release: `git tag v<x.y.z> && git push origin v<x.y.z>`

## Writing

Give human-facing prose a final readability pass. Lead with the outcome, use concrete plain language and active voice, and cut filler, stock framing, repetition, and promotional claims. Preserve meaning, evidence, citations, uncertainty, and established terminology. Never rewrite exact quotations, commands, logs, identifiers, API names, or contractual language. Match the audience and use restrained formatting.

## Pull requests

- Never create a pull request unless the developer explicitly asks.
- Use a plain-language Conventional Commit title. In the body, explain the problem before the solution and end with the required AI disclosure, naming the exact model, harness, and tooling used.
- Keep one concern per pull request. If the description needs the word "also" for another change, split it.
- When babysitting a pull request, poll for checks and comments newer than the last push. Verify bot findings against the source, fix real issues, and dismiss false positives with a written reason. Stay quiet when nothing is new, and stop when the latest commit is green.