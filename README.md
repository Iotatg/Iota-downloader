<p align="center">
  <img src="https://capsule-render.vercel.app/api?type=waving&color=gradient&customColorList=6,11,19,26&height=220&section=header&text=Iota%20Downloader&fontSize=60&fontColor=ffffff&animation=fadeIn&fontAlignY=38&desc=Go%20Telegram%20Media%20Downloader%20Bot&descAlignY=60&descAlign=50" width="100%"/>
</p>

<p align="center">
  <img src="https://readme-typing-svg.demolab.com?font=Fira+Code&weight=600&size=20&pause=1000&color=00E5FF&center=true&vCenter=true&width=620&lines=Go+Telegram+Media+Downloader+Bot;MTProto+Delivery+Of+The+Big+Files;Multi+Platform+Resolver+And+Audio+Extraction;Per+User+Quality%2C+Settings+And+Admin+Panel;Built+With+Go+And+PostgreSQL" alt="Typing SVG" />
</p>

<p align="center">
  <a href="https://github.com/Iotatg/Iota-downloader/stargazers">
    <img src="https://img.shields.io/badge/GitHub-Stars-FFD700?style=for-the-badge&logo=github&logoColor=white" alt="GitHub Stars">
  </a>
  <a href="https://github.com/Iotatg/Iota-downloader/fork">
    <img src="https://img.shields.io/badge/GitHub-Forks-00C853?style=for-the-badge&logo=github&logoColor=white" alt="GitHub Forks">
  </a>
  <a href="https://go.dev/">
    <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go Version">
  </a>
  <a href="https://github.com/Iotatg/Iota-downloader/commits/main">
    <img src="https://img.shields.io/badge/Build-passing-4CAF50?style=for-the-badge&logo=github&logoColor=white" alt="Build">
  </a>
  <a href="https://github.com/Iotatg/Iota-downloader/blob/main/LICENCE">
    <img src="https://img.shields.io/badge/License-GPL--3.0-blue?style=for-the-badge&logo=gnu&logoColor=white" alt="License">
  </a>
  <a href="https://t.me/Iota_QuotLyBot">
    <img src="https://img.shields.io/badge/Telegram-Bot-26A69A?style=for-the-badge&logo=telegram&logoColor=white" alt="Telegram Bot">
  </a>
</p>

---

## Features

- Direct downloads from a chat, a batch of up to six links per message.
- Playlists, albums, boards and radio mixes are expanded into a menu, the
  entries are picked one by one or the whole list is sent at once.
- Album, carousel and multi media posts.
- Audio extraction with the metadata of the artist, MP3, M4A and FLAC.
- Per user settings: video quality, send as file, audio format, descriptions,
  info buttons, URL button, MP3 button, file button and message clean up.
- Inline mode, `@YourBot <link>` delivers the media straight into the chat.
- Guest mode inside groups, a mention of the bot with a link starts a download.
- Adaptive download queue with backpressure, per user limits and a cache.
- Statistics screen with a rendered chart of the week, the month or the year.
- PostgreSQL storage of the users, the groups, the settings, the cached files,
  the analytics and the download history.
- Anti flood protection, request deduplication and an admin panel.
- Cover thumbnails on every delivered file and an embedded cover on the audio
  tracks.
- Video quality presets up to 4K, `1080p (best)`, `720p (balanced)`,
  `480p (data saver)` and `2160p (4k ultra)`.
- Files up to 2000 MB are delivered in one message, the bot talks to Telegram
  over MTProto and needs no local Bot API server. Bigger files are split into
  parts.
- Colourful buttons, every inline button carries its own accent glyph and one
  of the native colours of the protocol, red, green or blue. The colours are
  sent over MTProto, so every client that knows them shows the button in its
  colour, `DISABLE_COLOUR=true` turns them off.

Supported platforms: TikTok, Instagram, Threads, YouTube, YouTube Music,
X / Twitter, SoundCloud, Spotify, Pinterest, Apple Music, JioSaavn, Deezer,
Gaana, Tidal, Dailymotion, Bluesky, Facebook, Reddit, Twitch, Kick, Bilibili,
Tumblr, LinkedIn, IMDb, MX Player, Snapchat, Mastodon and Resso.

---

## Requirements

- Go 1.25 or newer
- PostgreSQL 14 or newer
- The identifier and the hash of a Telegram application, both come from
  [my.telegram.org/apps](https://my.telegram.org/apps)
- `yt-dlp` and `ffmpeg` on the `PATH` for YouTube, the video platforms of the
  social networks and the audio extraction
- `yt-dlp` with `curl_cffi` for the platforms that need an impersonation,
  Dailymotion for example:

```bash
# Install the impersonation support of yt-dlp
pip3 install --break-system-packages "curl_cffi>=0.11"
```

---

## Quick Deploy

```bash
# Clone the repository
git clone https://github.com/Iotatg/Iota-downloader.git
cd Iota-downloader

# Install the dependencies
go mod download

# Copy the environment template and fill in the values
cp .env.example .env
nano .env

# Create the folder of the MTProto session
mkdir -p session

# Create the database
createdb iota_downloader

# Run the bot
go build -o iota-bot .
./iota-bot
```

With Docker:

```bash
cp .env.example .env
# edit .env and set BOT_TOKEN, ADMIN_ID and the api credentials
docker compose up -d --build
```

The bot creates and migrates its own schema on the first start.

---

## Configuration

| Variable | Description |
| --- | --- |
| `BOT_TOKEN` | Telegram bot token |
| `ADMIN_ID` | Numeric Telegram user id of the bot owner |
| `ADMIN_IDS` | Comma separated list of additional admin ids |
| `DATABASE_URL` | PostgreSQL connection string |
| `API_ID` | Identifier of the Telegram application of the bot, `my.telegram.org/apps` |
| `API_HASH` | Hash of the Telegram application of the bot |
| `MTPROTO_SESSION` | File that keeps the MTProto session of the bot, `session/iota` by default |
| `COBALT_API_URL` | Cobalt backend used for Pinterest and SoundCloud |
| `INSTAGRAM_SESSIONID` | Session cookie of a logged in Instagram session, it opens Threads as well |
| `INSTAGRAM_COOKIES_FILE` | Netscape cookie file exported from a logged in Instagram session |
| `SPOTIFY_CLIENT_ID` / `SPOTIFY_CLIENT_SECRET` | Spotify Web API credentials |
| `IOTA_API_URL` / `IOTA_API_KEY` | Own iota-api instance used as the Spotify and SoundCloud backend |
| `RITESH_API_URL` / `RITESH_API_KEY` | Ritesh api, the multi platform resolver of the bot |
| `YTDLP_COOKIES_FILE` | Netscape cookie file used by every yt-dlp call, Reddit and Facebook ask for a session |
| `MTPROTO_FILE_LIMIT_MB` | Size limit of a single upload over MTProto, `2000` by default |
| `MAX_FILE_SIZE` | Size limit of a single download in bytes, `1.5 GB` by default |
| `SPLIT_LARGE_FILES` | Split the files that are too big into parts, `true` by default |
| `RESOLUTION_CACHE_TTL_SECONDS` | Seconds a resolved link is cached, `300` by default |
| `RESOLUTION_CACHE_MAX_ENTRIES` | Number of cached links, `500` by default |
| `COLLECTION_MAX_ITEMS` | Number of the entries a playlist, an album or a board is expanded into, `50` by default |
| `COLLECTION_PAGE_SIZE` | Number of the entries the menu of a collection shows at once, `10` by default |
| `COLLECTION_CACHE_TTL_SECONDS` | Seconds the menu of a collection stays alive, `1800` by default |
| `COLLECTION_CACHE_MAX_ENTRIES` | Number of the collections the bot keeps in memory, `50` by default |

Every optional variable is documented in `.env.example`.

### The video platforms of the social networks

Bluesky, Facebook, Reddit, Twitch, Kick, Bilibili, Tumblr, LinkedIn, IMDb,
MX Player and Snapchat are resolved with `yt-dlp`, the media of those
platforms is a protected stream and the download runs through `yt-dlp` as
well. Reddit and Facebook answer anonymous requests with a login wall, a
cookie file of a logged in session makes them work:

```bash
# Export the cookies of a logged in browser session in the netscape format
YTDLP_COOKIES_FILE=/app/cookies/platforms.txt
```

Mastodon is read from the public api of its instance, the open graph tags of
the page are the fallback for the instances that ask for a session. Resso is
served by the own iota-api (`rs:` media id), the api looks the track in the
Resso catalog and proxies the audio of the matching video.

### Speed

The bot answers a link with a status message right away and works on the
download in the background. Two caches keep the follow up requests fast:

- The resolution cache remembers a resolved link for
  `RESOLUTION_CACHE_TTL_SECONDS` seconds, so a link that was sent twice does
  not ask the platform again.
- The api caches the Spotify to YouTube mapping and the search results.

The timings of every step are written to `logs/perf_log.jsonl` as
`{"perf":"resolve","service":"spotify","duration_ms":186}`.

A host that builds a file on demand answers late. The downloader waits for the
first byte of the answer, it does not give up when the size of the file is
unknown, and it stops a download that stays silent for a minute.

### Spotify and SoundCloud through your own iota-api

Spotify has no public download link and SoundCloud needs a resolver backend
that is often unreachable. An own `iota-api` instance solves both: it resolves
the track on the source platform and proxies the audio, the bot only downloads
the stream.

```bash
# Mint a key on the api
curl -X POST http://127.0.0.1:8000/admin/keys \
  -H "X-Admin-Token: $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "iota-downloader"}'

# Point the bot at the api
IOTA_API_URL=http://127.0.0.1:8000
IOTA_API_KEY=iota_...
```

The bot then uses the api for every Spotify link (`sp:` media id) and searches
the api for the SoundCloud track (`/search`). Without `IOTA_API_URL` the bot
falls back to Cobalt for SoundCloud, and a Spotify link without credentials
cannot deliver an audio file.

The api resolves a Spotify track on YouTube, so it needs a cookie jar, a
datacenter ip is refused by YouTube without one:

```bash
# The api reads the jar of the bot, the file is refreshed by the bot itself
YOUTUBE_COOKIES_FILE=/workspace/Iota-downloader/cookies/youtube.txt
```

When the api is offline the bot does not answer with a bare "no media found",
it names the missing backend, so the admin sees what has to be started.

### The Ritesh api

The Ritesh api is a multi platform resolver that delivers the media of
YouTube, Spotify, Apple Music, JioSaavn, Deezer, Gaana, Tidal and
Dailymotion. The bot uses it in two ways:

1. It is the fallback of the platforms whose own resolver is blocked, a
   YouTube link that `yt-dlp` cannot answer is asked at the api.
2. It serves the music platforms that have no public download link, the
   Apple Music, JioSaavn, Deezer, Gaana and Tidal links are answered by the
   api alone. A Dailymotion link stays with `yt-dlp`, the api serves it only
   when `yt-dlp` fails.

A SoundCloud track is searched at the api as well, when the own iota-api
cannot find it and the api does not know the track. The title of the track is
read from the url then.

```bash
# The public api with its free key
RITESH_API_URL=https://api.riteshyt.in

# An own instance with its own key
RITESH_API_URL=https://api.example.com
RITESH_API_KEY=ritesh_...
```

The provider stays switched off while neither an address nor a key is set.

### Instagram needs a session

Instagram answers anonymous requests with a login wall, so the resolver needs a
session of a logged in account. Two options exist:

1. Copy the value of the `sessionid` cookie of a logged in browser session
   into `INSTAGRAM_SESSIONID`.
2. Export the whole cookie jar of `instagram.com` into a Netscape cookie file
   and point `INSTAGRAM_COOKIES_FILE` to it. The bot then reads the
   `sessionid` and `csftoken` cookies from the file and hands the file to
   `yt-dlp` as a fallback.

Without a session the bot answers with a short hint instead of a generic
error, because the link itself is fine and only the server side session is
missing.

### Playlists, albums and boards

A link that holds many entries is expanded into a menu instead of a download.
The bot recognises the playlists of Spotify, SoundCloud, Apple Music, Deezer,
JioSaavn, Gaana and Tidal, the albums of Spotify, Apple Music, Deezer and
JioSaavn, the radio mixes of YouTube and the boards of Pinterest.

The menu shows ten entries at a time, the pages are walked with the arrows and
the whole list is sent with the button `send all`. An entry is delivered with
the normal pipeline of the bot, so a track of a playlist is delivered exactly
like a track that was sent on its own.

The engines of the bot are asked one after the other, the first one that
answers delivers the entries. The api of the bot knows the playlists of
YouTube and of Deezer, the open api of Deezer reads a playlist without a key,
the Spotify Web API reads a playlist with the credentials of an application
and `yt-dlp` reads every list that the platform of `yt-dlp` knows. A platform
without a reader of its own answers with a short hint and asks for a single
track link.

The number of the entries of a list is limited by `COLLECTION_MAX_ITEMS`, a
list that holds more entries is cut and the menu tells how many entries of the
whole list are on the table.

---

## Big files

The bot talks to Telegram over MTProto, the protocol of the clients, and not
over the Bot API of the bots. The Bot API refuses every upload that is bigger
than 50 MB, a 4K video or a long audio track therefore never arrives. Over
MTProto the limit is 2000 MB per message, the bot delivers such a file in one
message.

A file that is bigger than the limit is cut into parts and the parts are sent
one by one, the caption of the first part holds the command that rebuilds the
file. The limit of a single part is 95 % of `MTPROTO_FILE_LIMIT_MB`, so the
server never refuses a part. The split can be switched off with
`SPLIT_LARGE_FILES=false`, a file that does not fit then never leaves the
server.

The quality preset of the user decides how big a video gets, a 1080p video of
an hour is bigger than 50 MB and is delivered in one message.

The presets of the settings menu are named after their resolution, `1080p
(best)`, `720p (balanced)`, `480p (data saver)` and `2160p (4k ultra)`. The
download asks for the height of the preset first, so a video that keeps the
wanted height only in a newer codec is not downgraded to the lower stream of
the older codec. A video without the wanted height arrives in the best
stream below it.

---

## Commands

| Command | Description |
| --- | --- |
| `/start` | Welcome message and the reply keyboard |
| `/help` | Supported sites and usage tips |
| `/settings` | The per user settings menu |
| `/stats` | Download statistics chart of the week, month or year |
| `/remove_keyboard` | Hides the reply keyboard |
| `/admin` | The admin panel (owner only) |
| `/perf` | Runtime metrics (owner only) |
| `/session` | Live session counters (owner only) |

---

## Project layout

```text
config/           environment configuration and validation
internal/app      wiring of every component and the update loop
internal/chart    statistics chart rendering (PNG, standard library only)
internal/download resilient downloader and adaptive queue
internal/handlers update processing and command handling
internal/keyboards telegram keyboards and callback data
internal/logger   structured file logging
internal/media    cache aware delivery pipeline
internal/messages all user facing texts
internal/platforms per platform resolvers and link detection
internal/runtime  deduplication, rate limiting and statistics
internal/services registry of the platform resolvers
internal/settings per user preferences
internal/storage  PostgreSQL schema and queries
internal/tg       Telegram client that speaks MTProto
internal/text     fancy unicode text styling
```

---

## License

GPL-3.0, see `LICENCE`. The bot uses [gogram](https://github.com/amarnathcjd/gogram)
for the MTProto connection, that library is published under the GPL-3.0 as
well.
