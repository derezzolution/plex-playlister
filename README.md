# plex-playlister

Golang service to expose a Plex Playlist using the Plex API.

![Screenshot](https://github.com/derezzolution/plex-playlister/blob/main/docs/screenshot.png)

## Usage

> [!IMPORTANT]
> Please remember to set the `keyCacheSalt` field in the config. Without this, a default **random** salt will be used
> and the checkbox state will not be accurately persisteded in local storage. Every service restart will result in a new
> salt and, thus, a new checkbox ID.

1. Copy `config.json.template` to `config.json` and populate with your specifics
1. Run with `go run .` to test your config locally

### Build and Package

1. Build with `./scripts/build.sh`
1. Package with `./scripts/package.sh`

### Deploy (general Linux distro)

1. Copy the package to your server (e.g. `scp plex-playlister.tar.gz my-server:/opt/`)
1. Connect to server (e.g. `ssh my-server`)
1. Change to the deploy directory (e.g. `cd /opt`)
1. Unpack the archive (e.g. `tar xvf plex-playlister.tar.gz`)
1. Create a systemd config (e.g. `vi /etc/systemd/system/plex-playlister.service`)

    ```systemd
    # Make sure to enable this Unit so it comes up on reboot with
    # `systemctl enable plex-playlister`

    [Unit]
    Description=plex-playlister
    After=network.target

    [Service]
    User=www-data
    Environment=GO_ENV=production
    TimeoutStartSec=0
    WorkingDirectory=/opt/plex-playlister
    ExecStart=/opt/plex-playlister/plex-playlister
    Restart=always
    RestartSec=1

    [Install]
    WantedBy=multi-user.target
    ```

1. Execute daemon reload (e.g. `systemctl daemon-reload`)
1. Start the plex-playlister service (e.g. `systemctl start plex-playlister`) and smoke test
1. Enable the plex-playlister service at boot (e.g. `systemctl enable plex-playlister`)

## Dev Notes

* **Quick Dev/Debug Template:** Use `python3 -m http.server 8080` and navigate to
  <http://localhost:8080/templates/playlist.html> to quickly iterate on the playlist template.

* **Need to tweak go-plex-client?** Commit change to <https://github.com/derezzolution/go-plex-client>. From this repo,
  pull the latest with `go get -u github.com/derezzolution/go-plex-client@4db197a` where `4db197a` is the latest commit.
