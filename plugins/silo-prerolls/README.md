# 🎬 Silo Prerolls

![Hero](assets/hero.png)

[![Build](https://github.com/apoapostolov/silo-prerolls/actions/workflows/build.yml/badge.svg)](https://github.com/apoapostolov/silo-prerolls/actions/workflows/build.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.26-blue.svg)](https://golang.org)
[![Silo Plugin SDK](https://img.shields.io/badge/Silo%20SDK-v0.30.0-green.svg)](https://github.com/Silo-Server/silo-plugin-sdk)

A **Silo plugin** that plays pre-roll videos before movies and TV episodes — bringing the cinema experience to your home media server.

Inspired by [Jellyfin's Cinema Mode](https://github.com/ZL154/jellyfin-projectionist) and [NeXroll](https://github.com/JFLXCLOUD/NeXroll), Silo Prerolls adds studio logos, seasonal intros, custom welcome screens, and trailer chains to your playback.

## ✨ Features

### Core
- **Folder-based discovery** — no library required, just point at a folder
- **5 selection strategies** — Random, Sequential, Weighted, Equal Rotation, Recency Boost
- **Sidecar metadata** — per-file `.json` files for tags, weights, ratings, schedules
- **Folder-level config** — `prerolls.json` for defaults and per-file overrides

### Smart Selection
- **Skip on resume** — replays from partial position skip the intro
- **Maturity gating** — preroll rating must not exceed the feature's rating
- **Cooldown** — don't replay the same preroll for the same movie (configurable hours)
- **Per-user rules** — all users, only specific users, or all except specific users
- **Session modes** — every playback, first of session, once per day, first of binge
- **Library rules** — match by library name, item type, require/exclude tags
- **Seasonal scheduling** — drop `halloween.mp4` or `xmas.mp4` for auto-trigger on dates
- **Opt-outs** — never preroll specific movies, series, or episodes

### Advanced
- **Skippable prerolls** — skip button overlay with configurable delay
- **Feature preload** — warmup playback info while prerolls run
- **Post-rolls** — outros play after the feature ends
- **Series-specific prerolls** — `theme-preroll.mp4` in show's extras folder
- **Trailer mode** — chain N local trailers from feature metadata
- **Coming-soon trailer** — prepend trailer from an unwatched movie
- **Loudness check** — ffmpeg volumedetect flags mismatched volume
- **Binge-friendly episodes** — auto-play next episode gets its own preroll

### Stats & Tracking
- Per-file playback counts + last-played timestamps
- Per-user playback counts
- Skip rate + average seconds-before-skip per preroll

### Integrations
- **NeXroll** — pull from NeXroll's library and active schedules (API key auth)
- **Local folders** — scan configured directories
- **Custom URLs** — download from any URL
- **Prerolls.me** — community library (manual download)

## 📸 Screenshots

> Screenshots will be added as the plugin UI is developed.

## 📦 Installation

### Build from source

```bash
git clone https://github.com/apoapostolov/silo-prerolls.git
cd silo-prerolls
go build -o silo-prerolls .
```

### Docker

```bash
docker build -t silo-prerolls .
docker run -p 8080:8080 silo-prerolls
```

### Install in Silo

1. Build the binary (or download from releases)
2. Upload to Silo via the plugin management UI
3. Configure preroll folders in the admin UI
4. Add preroll video files to configured folders
5. Enable Cinema Trailers in client settings

## ⚙️ Configuration

```json
{
  "enabled": true,
  "folders": [
    { "path": "/media/prerolls", "default_tags": ["general"] }
  ],
  "triggers": {
    "movies": true,
    "episodes": true,
    "music_videos": false
  },
  "selection_mode": "random",
  "cooldown_hours": 12,
  "session_mode": "every_playback",
  "maturity_gating": true,
  "skippable": true,
  "skip_delay_seconds": 5,
  "feature_preload": true,
  "post_rolls_enabled": false,
  "trailer_mode": false,
  "trailer_count": 2,
  "series_specific_prerolls": true,
  "series_preroll_filename": "theme-preroll",
  "allowed_formats": [".mp4", ".mkv", ".mov", ".webm", ".avi", ".m4v"],
  "opt_outs": [],
  "per_user_rules": {},
  "library_rules": []
}
```

### Selection Modes

| Mode | Description |
|------|-------------|
| `random` | Random selection (default) |
| `sequential` | Round-robin through all prerolls |
| `weighted` | Honour the `weight` field |
| `equal_rotation` | Strict fairness — fewest plays first |
| `recency_boost` | 2× weight for files added in the last 7 days |

### Session Modes

| Mode | Description |
|------|-------------|
| `every_playback` | Play a preroll on every playback |
| `first_of_session` | Only on the first playback of a session |
| `once_per_day` | Once per calendar day |
| `first_of_binge` | First episode of a series binge |

## 📁 Sidecar Metadata

### Per-file: `myfile.mp4.json`

```json
{
  "tags": ["halloween", "horror"],
  "weight": 2,
  "rating": "PG-13",
  "schedule": { "start": "2026-10-01", "end": "2026-11-01" }
}
```

### Folder-level: `prerolls.json`

```json
{
  "defaults": { "tags": ["general"], "weight": 1 },
  "files": {
    "intro1.mp4": { "tags": ["action"], "weight": 3 },
    "intro2.mp4": { "tags": ["comedy"], "weight": 1 }
  }
}
```

## 🔌 API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v2/plugins/prerolls/config` | GET | Get configuration |
| `/api/v2/plugins/prerolls/config` | PUT | Update configuration |
| `/api/v2/plugins/prerolls/folders` | GET | List preroll folders |
| `/api/v2/plugins/prerolls/folders` | POST | Add preroll folder |
| `/api/v2/plugins/prerolls/folders/{id}` | DELETE | Remove preroll folder |
| `/api/v2/plugins/prerolls/scan` | POST | Trigger folder rescan |
| `/api/v2/plugins/prerolls/stats` | GET | Playback stats |
| `/api/v2/plugins/prerolls/skip` | POST | Record a skip |
| `/api/v2/plugins/prerolls/discover` | GET | Discover preroll for an item |

## 🔗 NeXroll Integration

NeXroll is a self-hosted preroll manager that runs alongside Silo. To integrate:

1. Install NeXroll (Docker: `jbrns/nexroll`, port 9393)
2. Generate an API key in NeXroll Settings → API Keys
3. Configure the NeXroll client in the plugin with the base URL and API key
4. The plugin will pull prerolls from NeXroll's library and active schedules

NeXroll features:
- Sequence builder with flow view
- Schedules (daily/weekly/monthly/yearly)
- Calendar view
- Community prerolls (1,800+ free)
- NeX-Up trailer downloader (via Radarr/Sonarr)
- Generator Studio (custom welcome screens/announcements)

## 🌐 Web Client Patch

The `web/prerolls.js` file provides a client-side patch for preroll playback. To use:

1. Host the script alongside your Silo instance
2. Inject `<script src="/plugins/prerolls/web/prerolls.js"></script>` into Silo's `index.html`
3. The script handles skip buttons, countdown, and stats recording

## 🧪 Testing

```bash
go test ./...
```

## 📄 License

MIT License — see [LICENSE](LICENSE) for details.

## 🙏 Acknowledgments

- [Jellyfin Projectionist](https://github.com/ZL154/jellyfin-projectionist) — inspiration for selection strategies and smart features
- [NeXroll](https://github.com/JFLXCLOUD/NeXroll) — preroll management and community library
- [Prerolls.me](https://www.prerolls.me) — free community preroll videos
- [Silo](https://github.com/Silo-Server/silo-server) — the media server this plugin extends