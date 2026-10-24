# silo-plugin-metadata-comicinfo

A [Silo](https://github.com/Silo-Server/silo-server) metadata provider plugin
for **western comics**. It recognizes a comic by the `ComicInfo.xml` file
embedded inside its CBZ/CBR archive and maps that metadata onto the Silo
library item: title, series, issue number, count, volume, summary, creators,
publisher, imprint, genres, tags, characters, teams, locations, age rating,
and page information.

When the archive has no usable metadata, the plugin falls back to a chain of
scrapers: a local Grand Comics Database (GCD) SQLite dump, ComicVine, Metron,
and filename parsing.

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
| 4 | Metron | REST API | `metron_token` (optional) |
| 5 | Filename | Offline heuristic | None |

Metron is optional and disabled by default. Enable it and add a token to
include it in the chain; it sits after GCD and ComicVine.

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
| `enable_metron` | off | Query Metron. |
| `metron_token` | — | Metron API token. |
| `enable_filename` | on | Parse series/issue/year from the filename. |

The GCD dump is published at https://www.comics.org/download/ and requires a
free comics.org account.

1. Sign in at https://www.comics.org.
2. Open https://www.comics.org/download/, choose **Private Use**, and copy the
   SQLite dump link into `gcd_dump_url`.
3. In the browser DevTools, open **Network**, reload any comics.org page, open
   the request headers, and copy the value of the **`sessionid`** cookie. Paste
   it into `gcd_dump_cookie` as `sessionid=<value>`. If the download returns
   403 (Cloudflare), paste the whole `Cookie` header instead, including
   `cf_clearance` and `csrftoken`.
4. Set `gcd_dump_path` to where the dump should be saved (for example
   `/config/gcd.db`) and `gcd_refresh_hours` to the refresh interval.

The dump is about 6 GB. The cookie is stored encrypted by Silo.


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
