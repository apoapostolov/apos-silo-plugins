# silo-plugin-metadata-comicinfo

A [Silo](https://github.com/Silo-Server/silo-server) metadata provider plugin
for **western comics**. It recognizes a comic by the `ComicInfo.xml` file
embedded inside its CBZ/CBR archive and maps that metadata onto the Silo
library item: title, series, issue number, count, volume, summary, creators,
publisher, imprint, genres, tags, characters, teams, locations, age rating,
and page information.

When the archive has no usable metadata, the plugin falls back to a chain of
scrapers: a local Grand Comics Database (GCD) SQLite dump, ComicVine, and
Metron as a distant last resort. Filename parsing fills gaps at the end.

## ComicInfo.xml

The `ComicInfo.xml` format originates from the ComicRack application and is
now governed by the [Anansi Project](https://github.com/anansi-project/comicinfo).
This plugin targets that schema. See the Anansi documentation for the full
field reference: https://anansi-project.github.io

## Provider chain

| Order | Source | Type | Requirements |
|------|--------|------|--------------|
| 1 | `ComicInfo.xml` | Embedded, offline | None |
| 2 | Grand Comics Database | Local SQLite dump | `gcd_dump_path` (about 6 GB) |
| 3 | ComicVine | REST API | `comicvine_api_key` (free) |
| 4 | Metron | REST API | `metron_token`, off by default |
| 5 | Filename | Offline heuristic | None |

Metron is deliberately last: it is only queried after GCD and ComicVine
return nothing, and it is disabled by default.

## Configuration

| Key | Default | Purpose |
|-----|---------|---------|
| `enable_gcd` | on | Use the local GCD dump. |
| `gcd_dump_path` | — | Path to the extracted GCD `.db` file. |
| `gcd_dump_url` | — | Optional authenticated download URL for the dump. |
| `gcd_dump_cookie` | — | Optional Cookie header for the dump download. |
| `gcd_refresh_hours` | 168 | Dump refresh interval when a URL is set. |
| `enable_comicvine` | on | Query ComicVine. |
| `comicvine_api_key` | — | Free ComicVine API key. |
| `enable_metron` | off | Query Metron as last resort. |
| `metron_token` | — | Metron API token. |
| `enable_filename` | on | Parse series/issue/year from the filename. |

The GCD dump is published at https://www.comics.org/download/ and requires a
free comics.org account. Download it once, extract the `.db` file, and point
`gcd_dump_path` at it. The plugin can also fetch the dump from an
authenticated URL on a schedule, mirroring the manga plugin's local-dump
mode.

## Contents

- `comicinfo/` — Go package that parses `ComicInfo.xml` into a typed struct,
  with helpers for creators, genres, tags, characters, and manga detection.
- `metadata/` — common metadata model and provider contract.
- `provider/` — scraper implementations: `gcd`, `comicvine`, `metron`,
  `filename`, plus the priority `Manager`.
- `cmd/comicinfo/` — small CLI that parses a `ComicInfo.xml` file and prints
  its metadata as JSON (useful for debugging and fixture generation).
- `manifest.json` — Silo plugin manifest declaring the `metadata_provider.v1`
  capability and configuration schema.

## Build

```sh
make build        # builds bin/comicinfo
make test         # runs the parser and provider tests
```

## Status

The `ComicInfo.xml` parser, the GCD/ComicVine/Metron providers, filename
fallback, and tests are functional. The Silo plugin runtime wrapper (gRPC
service wiring against the Silo plugin SDK) is the next step; the manifest
and capability shape are already aligned with `metadata_provider.v1`.

## Metadata sources research

See [docs/metadata-sources.md](docs/metadata-sources.md) for the full survey:
ComicVine, Grand Comics Database (GCD), Metron, filename parsing, and
reference implementations (ComicTagger, comicbox, komf).

## License

MIT. See [LICENSE](LICENSE).

ComicInfo.xml schema copyright belongs to the Anansi Project (MIT).
