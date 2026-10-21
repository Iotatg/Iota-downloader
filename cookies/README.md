# Cookies

This folder holds the session cookies that the bot needs for the platforms
that ask for a login. The files inside are secrets and are never pushed to
the repository, the `.gitignore` of the project keeps them out of git.

## YouTube

```bash
# Export the cookies of a logged in browser session in the netscape format
# and save them as youtube.txt inside this folder
YTDLP_YOUTUBE_COOKIES_FILE=cookies/youtube.txt
```

The cookie file gives the bot the dash streams of YouTube, without it only
the progressive files of the smallest resolution are available.

## Instagram

```bash
# The whole cookie jar of instagram.com in the netscape format
INSTAGRAM_COOKIES_FILE=cookies/instagram.txt
```

## The other platforms

Reddit, Facebook, TikTok, Twitter, Threads and the video platforms of the
social networks use the same file, the path is set with
`YTDLP_COOKIES_FILE`.
