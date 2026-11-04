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

To start honk at login, turn on **Settings → Start at login**, or run `make login`. To stop that, turn it off, or run `make logout`.

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
| 👥 team review | Someone requested a review from your team. This does not honk unless you turn on **Settings → Honk for team reviews**. |

**Your own PR waits on you.** These chirp.

| Action | When |
|---|---|
| ⚔️ fix conflict | The PR has a merge conflict. |
| 🪳 fix tests | CI failed on the head commit. |
| 🪚 respond | A reviewer requested changes on the head commit. After you push, the turn goes back to the reviewer. |
| 🚀 merge | The PR is approved and CI is green. |

honk skips drafts, and PRs with no update for 60 days (**Settings → Skip PRs idle for**).

## When it honks

honk polls every 2 minutes (**Settings → Check GitHub every**). It makes a sound only for an action that it did not see before. One poll makes one sound and one notification, even when it finds many actions. A honk wins over a chirp.

While PRs still wait on you, honk reminds you again every 2 hours. A reminder makes the same sound and says "still wait on you". By default, reminders only come Monday to Friday, from 9 AM to 6 PM local time. So the first poll after 9 AM on a workday reminds you of what waited overnight. Any honk or chirp starts the 2 hours again.

State, logs, and the sound files are in `~/Library/Application Support/honk/`. Delete `state.json` to hear every action again.

The first time honk.app starts, macOS asks if honk can send notifications. Choose **Allow** in the prompt; a click on the banner itself does not count. If you miss the prompt, turn honk on in System Settings → Notifications. A click on a notification about one PR opens that PR. A click on a notification about several PRs opens honk's menu, which lists them all. This works only when honk runs from honk.app.

## Settings

Change everything in honk's menu, under **Settings**. A change works at once. honk saves it in `~/Library/Application Support/honk/config.json`.

| Setting | Default | |
|---|---|---|
| Check GitHub every | 2 minutes | Time between polls. |
| Remind me every | 2 hours | How often honk reminds you about PRs that still wait. **Never** turns reminders off. |
| Remind on | Weekdays | The days that reminders come. Turn each day on or off. |
| Workday starts at, Workday ends at | 9 AM, 6 PM | The hours that reminders come. Choose **Midnight** for both to get reminders all day. |
| Skip PRs idle for | 60 days | Skip PRs with no update for this long. **Never** keeps all. |
| Skip orgs | None | The orgs of your PRs. A check mark makes honk skip that org. |
| Honk for team reviews | Off | Honk, and not only list, when someone asks your team for a review. |
| Start at login | | Adds or removes the LaunchAgent that starts honk at login. |

Two flags help when you test: `honk -check` prints what waits on you, then exits. `honk -test-sound` plays both sounds.

## Code

| File | Job |
|---|---|
| `github.go` | Gets the token, runs the three GraphQL searches and the notifications call, and tries again after a 5xx. |
| `rules.go` | `Decide` turns a snapshot into actions. It is a pure function; `rules_test.go` covers it. |
| `state.go` | Remembers the actions that it saw, so a restart does not honk again. |
| `sound.go` | Embeds the honk, makes the chirp in code, and plays them with `afplay` (or `paplay`/`aplay` on Linux). |
| `notify.go` | Writes the notification text and opens PR links. |
| `remind.go` | Decides when a reminder is due. `remind_test.go` covers it. |
| `config.go` | Loads, cleans, and saves the settings in `config.json`. |
| `settings.go` | Builds the Settings menu. |
| `login_darwin.go` | Writes or removes the LaunchAgent for Start at login. |
| `notify_darwin.go`, `notify_darwin.m` | Posts native macOS notifications from honk.app. A click opens the PR. Outside the bundle, it falls back to `osascript`. |
| `notify_other.go` | Posts notifications with `notify-send` on Linux and BSD. |
| `tools/icon.swift` | Draws the app icon: the goose emoji from the system font on a pond-blue squircle. `make app` runs it, so the repo holds no emoji art. |
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
