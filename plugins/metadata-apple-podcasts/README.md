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
