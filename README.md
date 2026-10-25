# Silo plugins

A monorepo of [Silo](https://github.com/Silo-Server/silo-server) plugins. Each
plugin is its own Go module under `plugins/` and ships one binary per supported
platform. The `repository.json` catalog lists every plugin, so a Silo server
installs and updates them from one repository URL.

## Repository URL

```
https://raw.githubusercontent.com/apoapostolov/apos-silo-plugins/main/repository.json
```

Add it under **Admin → Plugins → Repositories**, then install any plugin from
the catalog.

## Plugins

| Plugin | ID | Version | Capability | Source |
|--------|----|---------|------------|--------|
| Trakt Watch Provider | `community.watchprovider.trakt` | 0.4.0 | `watch_sync_provider.v1` | [`plugins/watchprovider-trakt`](plugins/watchprovider-trakt) |
| ComicInfo Metadata | `silo.comicinfo-metadata` | 0.1.2 | `metadata_provider.v1` | [`plugins/metadata-comicinfo`](plugins/metadata-comicinfo) |
| Apple Podcasts Metadata | `silo.apple-podcasts-metadata` | 0.2.0 | `metadata_provider.v1`, `image_resolver.v1` | [`plugins/metadata-apple-podcasts`](plugins/metadata-apple-podcasts) |

## Build

Each plugin builds independently:

```bash
cd plugins/watchprovider-trakt && make test && make build-all
cd plugins/metadata-comicinfo    && make test && make build-all
cd plugins/metadata-apple-podcasts && make test && make build-all
```

`make build-all` writes `dist/plugin-<os>-<arch>` for linux/amd64,
linux/arm64, and darwin/arm64. Releases publish those binaries under
per-plugin asset names (`trakt-*`, `comicinfo-*`).

## Add a plugin

1. Add `plugins/<name>/` with its own `go.mod`, `manifest.json`, and `main.go`
   that embeds the manifest and calls `runtime.ServeManifestWithOptions`.
2. Add an entry to `repository.json` with the manifest and one binary
   URL + SHA-256 checksum per platform.
3. Attach the built binaries to the next release with a unique asset prefix.
