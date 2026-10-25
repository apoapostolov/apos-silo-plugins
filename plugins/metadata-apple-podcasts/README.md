# Apple Podcasts Metadata

A Silo metadata provider and artwork resolver for podcast shows, backed by the
Apple Podcasts directory and each show's own RSS feed.

```text
plugin_id       silo.apple-podcasts-metadata
capabilities    metadata_provider.v1 (apple-podcasts)
                image_resolver.v1 (apple-podcasts)
```

## What it fills

Matching a show against the Apple directory, then reading its feed, gives Silo:

| Field | Source |
| --- | --- |
| Title, original title | Directory |
| Year, first air date | Directory release date |
| Latest episode date | Newest feed item |
| Description | Feed channel description, markup stripped |
| Publisher | Feed `itunes:author`, falling back to the directory artist; written to studios |
| Genres | Directory genres plus feed categories |
| Language | Feed language, falling back to the directory |
| Content rating | `Explicit` when the directory or feed says so |
| Poster | Apple cover, served through the image resolver at the size Silo asks for |

## Artwork

The stored path is `apple-podcasts://<collection-id>/poster.jpg`. The image
resolver capability owns that scheme and rewrites Apple's size segment
(`100x100bb` → `1200x1200bb`) for the host's variant hint: `card` 400,
`featured` 800, `large` 1200, `full` 2000, `original` 3000. An unknown variant
degrades to 800 instead of failing. That is one image file served at any size by
Apple's own CDN, so no re-hosting is involved.

## Matching

Apple's search is ranked by popularity, so an exact query for a long on-disk
title often returns nothing at all while a shorter clause returns the show. The
provider therefore:

- searches several query shapes (the whole title, the clause before a dash,
  colon or pipe, the first three identifying words, the longest word) and merges
  the results, widening to the `us`, `gb`, `ca` and `au` storefronts when the
  first pass finds nothing;
- drops format words before comparing (actual play, D&D, 5E, dark fantasy,
  podcast, official, …), because those appear on half the shows in the category;
- scores every candidate against the original title with word containment plus
  character-bigram overlap, tolerating plurals like `Turncloak`/`Turncloaks`;
- keeps only candidates above 0.42 confidence, best first.

The match dialog gets the confidence as the first line of each candidate's
description (`Apple match 94% · Rusty Quill · 401 episodes · Performing Arts`),
since the host has no score column.

A show that Apple has de-listed cannot be matched by any query. When that
happens the honest answer is "not in the directory", not "the plugin is broken";
the feed path is the way to fill in such a show.

## Install

Add this repository URL under **Admin → Plugins → Repositories** in Silo:

```text
https://raw.githubusercontent.com/apoapostolov/apos-silo-plugins/main/repository.json
```

Install **Apple Podcasts Metadata** from the catalog, then add **Apple Podcasts**
to a podcast library's metadata provider chain.

## Configuration

| Key | Default | Meaning |
| --- | --- | --- |
| `country` | `us` | Two-letter Apple storefront to search: `us`, `gb`, `de`, `bg`, … |

No account, key, or token is needed. The directory API and the feeds are public.

## Build

```bash
make test
make build-all      # dist/plugin-linux-amd64, -linux-arm64, -darwin-arm64
```

```bash
# Unit tests use an httptest directory and feed.
make test

# The live tests call Apple and a real feed.
go test -tags live -run Live ./provider/apple
```

## Limits

- **The host must route podcasts to metadata providers.** In current Silo,
  `ContentLevelsForLibraryType("podcasts")` returns no content level, so a
  podcast library has no provider chain to attach to and the scanner writes no
  artwork. The provider is correct against `metadata_provider.v1` and starts
  being called the moment podcast libraries serve a content level.
- The directory is per storefront. A show carried only in one storefront is
  found only when `country` matches it.
- Directory episode counts are a snapshot; the feed is the live count.
- A self-published show with no Apple listing cannot be matched. Search returns
  nothing rather than guessing.

## License

MIT. See [LICENSE](LICENSE).
