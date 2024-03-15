# plex-playlister

Golang service to expose a Plex Playlist using the Plex API.

## Notes

* **Quick Dev/Debug Template:** Use `python3 -m http.server 8080` and navigate to
  <http://localhost:8080/templates/playlist.html> to quickly iterate on the playlist template.

* **Need to tweak go-plex-client?** Commit change to <https://github.com/derezzolution/go-plex-client>. From this repo,
  pull the latest with `go get -u github.com/derezzolution/go-plex-client@4db197a` where `4db197a` is the latest commit.

## Usage

> [!IMPORTANT]
> Please remember to set the `keyCacheSalt` field in the config. Without this, a default **random** salt will be used
> and the checkbox state will not be accurately persisteded in local storage. Every service restart will result in a new
> salt and, thus, a new checkbox ID.

1. Copy `config.json.template` to `config.json` and populate with your specifics

1. Run with `go run .`

## Screenshot

> [!IMPORTANT]
> Sorry this is totes uggo. 😅  Style updates coming soon...

![Screenshot](https://github.com/derezzolution/plex-playlister/blob/main/docs/screenshot.png)
