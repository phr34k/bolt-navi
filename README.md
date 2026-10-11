# bolt-navi

**A keyboard-driven launcher for Puppet Bolt tasks and plans.**

Stop memorising task names and parameter flags. bolt-navi gives you a fuzzy
search palette over every task and plan in your project, opens a guided
parameter wizard with live command preview, and runs bolt — all without leaving
your terminal.

Ships as a single static binary. No runtime, no dependencies, no install step
beyond dropping the file on your PATH.

---

## Features

- **Fuzzy search** — type any fragment of a task or plan name and results rank
  instantly with matched characters highlighted.
- **Run wizard** — every parameter on one focused screen: text fields, enum
  dropdowns, boolean toggles. Required fields are flagged; the command is
  previewed live as you type.
- **Target picker** — checkbox multi-select over your inventory targets and
  groups, loaded straight from `inventory.yaml`.
- **Live output** — bolt runs in the foreground with full output visible; the
  TUI waits for you to press Enter before repainting so nothing scrolls away.
- **Shell escape hatch** — `Ctrl+T` drops into an interactive shell and carries
  environment variables (e.g. `$VAULT_TOKEN`) back into the session when you
  return.
- **One binary, everywhere** — statically compiled; works on Windows, macOS,
  and Linux with identical behaviour.

---

## Installation

Download the binary for your platform from the
[releases page](https://github.com/phr34k/bolt-navi/releases) and place it
anywhere on your `PATH`.

Or build from source (requires Go 1.21+):

```sh
go install github.com/phr34k/bolt-navi@latest
```

`bolt` must be on your `PATH`. Both [Puppet Bolt](https://www.puppet.com/docs/bolt) and
[OpenBolt](https://github.com/OpenVoxProject/openbolt) are supported — they share
the same executable name and project layout.

---

## Usage

```sh
# Run from inside a Bolt project directory
bolt-navi

# Point at a project from anywhere
bolt-navi --project C:\ops\bolt
bolt-navi --project /home/ops/bolt

# Pre-populate the targets field
bolt-navi --targets web1,web2
bolt-navi --targets all
```

| Key | Action |
|---|---|
| Type | Filter tasks & plans |
| `↑` / `↓` | Move selection |
| `Enter` | Open run wizard |
| `Ctrl+R` | Run immediately (wizard) |
| `Space` | Open target picker (targets field) |
| `Ctrl+T` | Drop into shell |
| `Esc` | Clear search / back / quit |

---

## Building from source

```sh
git clone https://github.com/phr34k/bolt-navi
cd bolt-navi
go build -o bolt-navi .
```

Cross-compile (fully static, no CGO):

```sh
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bolt-navi.exe .
CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -o bolt-navi     .
CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -o bolt-navi-mac .
```

---

## License

Apache License 2.0 with Additional Conditions — see [LICENSE](LICENSE).  
Redistribution must preserve the donation mechanism unchanged.

---

## Support

bolt-navi is free and open-source. If it saves you time, please consider
[supporting the project](https://github.com/sponsors/phr34k). ♥
