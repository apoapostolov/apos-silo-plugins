# Metadata sources for western comics

Research into scrapers and metadata providers that could feed the
`comicinfo-metadata` Silo plugin, beyond the `ComicInfo.xml` already
embedded in the archive.

## Local / offline (no API key, already supported)

- **ComicInfo.xml** (Anansi schema) — embedded in CBZ/CBR. Already parsed
  by `comicinfo/`. Primary source.
- **MetronInfo.xml** — alternative embedded format (Metron project).
- **ComicBookInfo** (Comic Book Lover) — archive comment JSON.
- **CoMet** — another embedded format.
- **Filename parsing** — series/issue/year can be inferred from the file name
  (ComicTagger/comicbox both do this).

## Online providers (require credentials, optional)

| Provider | Used by | API | Notes |
|---|---|---|---|
| ComicVine | ComicTagger, ComicRack (Comic Vine Scraper), Mylar, komf, comicbox | REST, free API key required; rate-limited (~200 req/hour) | De-facto standard for western comics; issue/volume matching. |
| Grand Comics Database (GCD) | strumples/ComicRack-Grand-Comics-Scraper | Downloadable SQLite dump + web API | Offline-capable via local dump; very complete for older comics. |
| Metron | comicbox | REST, API token | Smaller but clean; token from metron.cloud account. |
| League of Comic Geeks | — | No public API | Scrape-only; fragile. |
| Bangumi | komf | REST, token | Chinese-language metadata. |

## Reference implementations to borrow from

- **comictagger/comictagger** (Python) — ComicVine matching heuristics,
  image-based cover matching, batch tagging. Source of the "comicinfoxml"
  plugin that writes Anansi ComicInfo.xml.
- **ajslater/comicbox** (Python) — unified metadata model across
  ComicInfo.xml/MetronInfo.xml/ComicBookInfo/CoMet/PDF; online tagging via
  Metron + ComicVine; filename parsing; cover extraction.
- **Snd-R/komf** (Kotlin) — Komga/Kavita fetcher; provider chain with
  priority, aggregation, field-level merge; ComicVine + GCD-adjacent.
- **strumples/ComicRack-Grand-Comics-Scraper** — offline GCD SQLite scraper.

## Recommended integration order

1. Keep `ComicInfo.xml` as the authoritative, offline source (done).
2. Add filename-based fallback for archives without embedded metadata.
3. Add optional ComicVine provider (API key in Silo plugin config), reusing
   ComicTagger's matching heuristics and respecting rate limits.
4. Add optional GCD provider via local SQLite dump for offline coverage.
5. Consider Metron as a secondary online source.

## Constraints

- Silo plugins run as gRPC binaries; network access must be declared via the
  `network_access_provider.v1` capability if the host requires it.
- ComicVine rate limits make batch tagging of large libraries slow; a
  background/queued approach is needed.
- GCD data dumps are large (~GBs); storage path must be configurable (mirror
  the manga plugin's `dump_path` pattern).

## Implementation status

Implemented in this repository:

- `provider/gcd` — reads a local GCD SQLite dump (series/issue/story/credits
  tables), with an optional authenticated dump download and refresh loop.
- `provider/comicvine` — search and issue fetch against the ComicVine API.
- `provider/metron` — search and issue fetch against the Metron API; wired
  last in the chain and off by default.
- `provider/filename` — series/issue/year fallback from archive names.
- `provider.Manager` — priority chain: GCD, ComicVine, Metron, filename.

Not implemented yet: the Silo gRPC runtime wrapper, and a GCD web-API client
(the local SQLite dump is used instead).
