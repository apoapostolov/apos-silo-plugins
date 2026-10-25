# silo-plugin-metadata-comicinfo

A [Silo](https://github.com/Silo-Server/silo-server) metadata provider plugin
for **mainstream comics**. It resolves a comic to its series and issue from a
local Grand Comics Database (GCD) SQLite dump, ComicVine, or Metron, falls back
to parsing the filename when none of them answer, and maps the result onto the
Silo library item: title, series, issue number, count, volume, summary,
creators, publisher, imprint, genres, tags, characters, teams, locations, age
rating, and page information.

## ComicInfo.xml

The `ComicInfo.xml` format originates from the ComicRack application and is
now governed by the [Anansi Project](https://github.com/anansi-project/comicinfo).
This repository contains a parser for it (`comicinfo/`), a CLI that prints a
parsed file as JSON (`cmd/comicinfo/`), and tests for both.

The running plugin does not use them yet. Silo's `metadata_provider.v1` request
carries a title, a year, and provider IDs, not the archive, so the plugin cannot
open a CBZ/CBR and read the file inside it. Reading embedded ComicInfo.xml needs
a capability that hands over the file; until one exists, the plugin is an online
scraper with a filename fallback.

## Provider chain

| Order | Source | Type | Requirements |
|------|--------|------|--------------|
| 1 | Grand Comics Database | Local SQLite dump | `gcd_dump_path` (about 6 GB) |
| 2 | ComicVine | REST API | `comicvine_api_key` (free) |
| 3 | Metron | REST API | `metron_token` (optional) |
| 4 | Filename | Offline heuristic | None |

Metron is optional and disabled by default. Enable it and add a token to
include it in the chain; it sits after GCD and ComicVine.

## Configuration

| Key | Default | Purpose |
|-----|---------|---------|
| `enable_gcd` | on | Use the local GCD dump. |
| `gcd_dump_path` | — | Path to the extracted GCD `.db` file. |
| `enable_comicvine` | on | Query ComicVine. |
| `comicvine_api_key` | — | Free ComicVine API key. |
| `enable_metron` | off | Query Metron. |
| `metron_token` | — | Metron API token. |
| `enable_filename` | on | Parse series/issue/year from the filename. |

The GCD dump is published at https://www.comics.org/download/ and requires a
free comics.org account.

1. Sign in at https://www.comics.org.
2. Open https://www.comics.org/download/ and choose **Private Use**.
3. Download the SQLite dump and unpack it.
4. Set `gcd_dump_path` to the `.db` file (for example `/config/gcd.db`).

The dump is about 6 GB, and refreshing it means downloading it again: the
plugin reads the file in place and never fetches it.


## Contents

- `comicinfo/` — Go package that parses `ComicInfo.xml` into a typed struct,
  with helpers for creators, genres, tags, characters, and manga detection.
  Used by the CLI today; the provider chain does not call it.
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

The Silo runtime wrapper is wired: the plugin installs, serves the
`metadata_provider.v1` capability, and resolves comics through the GCD,
ComicVine, Metron, and filename providers.

Two limits are worth knowing before installing it. Embedded `ComicInfo.xml` is
parsed by the CLI but not read from archives, because the metadata capability
never receives the file. And the plugin never downloads the GCD dump: it opens
the file at `gcd_dump_path`, so the dump is fetched and refreshed by hand.

## Metadata sources research

See [docs/metadata-sources.md](docs/metadata-sources.md) for the full survey:
ComicVine, Grand Comics Database (GCD), Metron, filename parsing, and
reference implementations (ComicTagger, comicbox, komf).

## License

MIT. See [LICENSE](LICENSE).

ComicInfo.xml schema copyright belongs to the Anansi Project (MIT).
