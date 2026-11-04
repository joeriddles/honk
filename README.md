# honk 🪿

A menubar app that honks when a pull request waits on you.

honk talks only to `api.github.com`. It uses your `gh` login or a personal access token. No other server sees your token or your PRs.

## Start

The sounds are in [Git LFS](https://git-lfs.com). Install it before you clone, or run `git lfs pull` after. Without LFS, `make test` fails, because the build would embed an LFS pointer and not the honk. For the same reason, `go install github.com/joeriddles/honk@latest` does not work.

```bash
make check    # print what waits on you, then exit
make sound    # play the honk and the chirp
make install  # build honk.app into ~/Applications
open ~/Applications/honk.app
```

To start honk at login, run `make login`. Add flags with `ARGS`, for example `make login ARGS="-exclude-orgs wildecare"`. To stop that, run `make logout`.

## Token

honk takes the first token that it finds:

1. `GITHUB_TOKEN`
2. `GH_TOKEN`
3. `gh auth token`

A fine-grained token needs read access to **Pull requests**, **Metadata**, and **Commit statuses** (for CI state). @mentions use the notifications API. That API needs a classic token with the `notifications` or `repo` scope, so a fine-grained token gets reviews but not mentions.

## Whose turn is it?

honk gives each PR one action at most. The first rule that matches wins.

**Someone waits on you.** These honk.

| Action | When |
|---|---|
| 👀 review | Someone requested your review by name. |
| 🔁 re-review | You requested changes, and the author pushed after your review. Your review still blocks the merge. |
| 💬 mention | An unread notification @mentions you on a PR. Open the PR and the notification goes away. |
| 👥 team review | Someone requested a review from your team. This does not honk unless you set `-team-honk`. |

**Your own PR waits on you.** These chirp.

| Action | When |
|---|---|
| ⚔️ fix conflict | The PR has a merge conflict. |
| 🪳 fix tests | CI failed on the head commit. |
| 🪚 respond | A reviewer requested changes on the head commit. After you push, the turn goes back to the reviewer. |
| 🚀 merge | The PR is approved and CI is green. |

honk skips drafts, and PRs with no update for 60 days (`-stale-days`).

## When it honks

honk polls every 2 minutes (`-interval`). It makes a sound only for an action that it did not see before. One poll makes one sound and one notification, even when it finds many actions. A honk wins over a chirp.

State, logs, and the sound files are in `~/Library/Application Support/honk/`. Delete `state.json` to hear every action again.

## Flags

| Flag | Default | |
|---|---|---|
| `-interval` | `2m` | Time between polls. The minimum is 30s. |
| `-stale-days` | `60` | Skip PRs with no update for this many days. `0` keeps all. |
| `-exclude-orgs` | | Orgs to skip, comma-separated. |
| `-team-honk` | `false` | Honk for team review requests too. |
| `-check` | | Print the actions once, then exit. |
| `-test-sound` | | Play both sounds, then exit. |

## Code

| File | Job |
|---|---|
| `github.go` | Gets the token, runs the three GraphQL searches and the notifications call, and tries again after a 5xx. |
| `rules.go` | `Decide` turns a snapshot into actions. It is a pure function; `rules_test.go` covers it. |
| `state.go` | Remembers the actions that it saw, so a restart does not honk again. |
| `sound.go` | Embeds the honk, makes the chirp in code, and plays them with `afplay` (or `paplay`/`aplay` on Linux). |
| `notify.go` | Posts notifications with `osascript` (or `notify-send`) and opens PR links. |
| `tray.go` | The menubar, built on [`fyne.io/systray`](https://github.com/fyne-io/systray). |

Each poll makes three GraphQL calls and one REST call. One poll costs about 6 points. At the default interval, that is about 180 of your 5,000 points an hour. A poll takes about 10 seconds.

## The honk

[`sounds/honk.wav`](sounds/honk.wav) is a real Canada goose (*Branta canadensis*). It comes from [`Branta_canadensis.ogg`](https://commons.wikimedia.org/wiki/File:Branta_canadensis.ogg) on Wikimedia Commons, a recording of the US National Park Service. The file is in the public domain.

To make the clip from the original, run:

```bash
ffmpeg -i Branta_canadensis.ogg \
  -af "atrim=2.15:2.85,asetpts=PTS-STARTPTS,highpass=f=200,afade=t=in:d=0.02,afade=t=out:st=0.62:d=0.08,volume=-0.7dB,aresample=22050" \
  -ac 1 -c:a pcm_s16le -map_metadata -1 -fflags +bitexact sounds/honk.wav
```
