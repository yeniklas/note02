# note02

A terminal note-taking app that keeps your notes **encrypted at rest** and **version-controlled in git**, so the remote never sees plaintext.

Each note is encrypted with [`age`](https://github.com/FiloSottile/age) (passphrase + scrypt) and stored as a single `<uuid>.age` file. Every create / update / delete is automatically committed and pushed to your git repo.

## Features

- **Encrypted storage** — one `age`-encrypted file per note; your passphrase is prompted at startup and never written to disk.
- **Git-backed sync** — notes are auto-committed and pushed after every change (push failure is non-fatal, so it still works offline).
- **Bubble Tea TUI** — list, preview, full-text search, and tag filtering.
- **External editor** — edit notes in `$EDITOR` (falls back to `vi`).
- **Markdown rendering** — optional [Glamour](https://github.com/charmbracelet/glamour) preview.
- **Journal mode** — `note02 --journal` opens (or creates) today's dated journal entry.

## Install

Download a pre-built binary from the [releases page](https://github.com/yeniklas/note02/releases), or install from source:

```sh
go install github.com/yeniklas/note02@latest
```

Requires **Go 1.25+** to build from source.

Check the version with `note02 --version`, and update an installed binary in place with `note02 --self-update`.

## Configuration

note02 reads `~/.config/note02/config.toml`:

```toml
[repo]
# A git repository directory where encrypted notes are stored (under notes/).
# Configure a remote here for push-sync to work.
path = "/home/you/notes-repo"

[display]
# Render note previews as markdown (default: true).
markdown = true

[journal]
# Tags applied to new journal entries (default: ["journal"]).
tags = ["journal"]
```

`repo.path` is required. Point it at a git repo with a remote configured if you want your encrypted notes pushed automatically.

## Usage

```sh
note02              # launch the TUI (prompts for your passphrase)
note02 --journal    # open or create today's journal entry in $EDITOR
```

## License

[MIT](LICENSE)
