# 0001 — Single instance via Unix socket; full window takes priority over mini player

## Context

Robsong should open audio files from the file manager ("Open with Robsong", double-click when it is the default app). One file opens a lightweight "mini" player (bottom bar only); several files open the full window with a temporary "Play Now" playlist.

Without coordination, every open would start a new process: two sounds playing over each other and two full windows writing to the same SQLite library (`~/.config/robsong/library.db`, no WAL, no busy timeout).

Rejected alternatives:

- **A new process on every open.** It is the simplest option, but it overlaps playback and runs concurrent writers on one DB.
- **A separate mini instance next to the full window.** The user explicitly rejected it: mini and full must never run at the same time.
- **D-Bus activation (`DBusActivatable`, `org.freedesktop.Application`).** It is the idiomatic Linux option, but it adds a dependency and session-bus wiring that Fyne does not provide. A Unix socket is enough for a Linux-only app.
- **The mini player hands over to a freshly started full process.** It interrupts playback and loses the mini's current track.

## Decision

- Robsong runs as **at most one process**. On start it tries to connect to a Unix socket (`$XDG_RUNTIME_DIR/robsong.sock`, falling back to the config dir).
  - If the connection succeeds, it sends the absolute file paths (possibly none) and exits.
  - If the connection fails, it removes any stale socket, listens, and becomes the primary instance.
- **The full window always takes priority:**
  - Nothing running: 1 track opens the mini, more than 1 track opens the full window with "Play Now".
  - Full window running: any open (1 or N files) replaces "Play Now", selects it, starts playing and raises the window.
  - Mini running, 1 file: the mini replaces its track.
  - Mini running, N files or a plain launch without files: the **same process** turns the mini into the full window. The audio player is shared, so playback continues. A plain launch carries the current mini track into "Play Now"; N files replace "Play Now".
- The mini's "Expand" button uses the same in-process mini→full transformation.

## Consequences

- `main` gains a client/server step before any UI or DB work. The mini start path must not open the SQLite store (it is opened lazily, see analysis).
- The UI layer must support switching windows inside one Fyne app while keeping one `audio.Player`. The player bar must be buildable without the full `App` state.
- Stale sockets (after a crash) must be detected and removed. Two processes starting at the same moment can still race; this is acceptable for a desktop launcher.
- Never running two instances also removes the multi-writer risk on the SQLite library.
- Moving to D-Bus later would replace only the transport; the priority rules stay the same.
