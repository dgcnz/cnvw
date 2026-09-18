# cnvw

A read-only terminal viewer for Obsidian / [JSON Canvas](https://jsoncanvas.org/spec/1.0/) `.canvas` files.

It behaves more like a two-dimensional `less` than a canvas editor: it preserves
the spatial layout, lets you pan, zoom and search, and never writes to the file.

```
          │                                         │   ╭─ Downstream ───────────╮
          │  ╭──────────────╮       ╭──────────────╮│   │   ╭─────────────────╮  │
          │  │ Ingest       │       │ Normalize    ││  ┌───▶│ ▤ notes/wareho… │  │
          │  │            … │──────▶│            … │───┘│   ╰─────────────────╯  │
          │  ╰──────────────╯       ╰──────────────╯│   │            │           │
          │          │                      │       │   │            ▼           │
          │          │                      │ failures  │   ╭─────────────────╮  │
          │          │  ╭──────────────╮    │       │   │   │ ↗ https://jsonc │  │
          │┌─ replay ┴─▶│ Dead letter… │◀───┘       │   │   ╰─────────────────╯  │
          ││            ╰──────────────╯            │   │            │           │
          ╰│────────────────────────────────────────╯   │            │           │
           │                                            │   ╭─────────────────╮  │
```

Full frames: [sample](docs/sample.txt), [grouped](docs/grouped.txt),
[labeled-tree](docs/labeled-tree.txt).

## Install

```sh
go install github.com/dgcnz/cnvw@latest
```

Go 1.25.8 or newer. That is the whole dependency list.

## Use

```sh
cnvw board.canvas
cnvw --theme light board.canvas
cnvw --vault ~/notes board.canvas    # resolve file nodes against a vault
cnvw --dump 120x40 board.canvas      # render one frame to stdout and exit
```

### Keys

| | |
|---|---|
| `h j k l`, arrows | pan |
| `ctrl+d` / `ctrl+u` | pan half a screen |
| `ctrl+f` / `ctrl+b` | pan a full screen |
| `H J K L` | jump to the nearest node that way |
| `tab` / `shift+tab` | cycle nodes in reading order |
| `g` / `G` | first / last node |
| `+` / `-` / `0` | zoom in / out / reset |
| `f` | fit the whole canvas |
| `z` | zoom to the selected node |
| `enter` | open the selected node in the reader |
| `/`, `n`, `N` | search, next match, previous match |
| `r` | reload the file from disk |
| `e` | toggle edge labels |
| `t` | toggle light and dark |
| `?` | help |
| `q` | quit |

The mouse works too: click to select, wheel to pan, ctrl+wheel to zoom.

## More

- [docs/design.md](docs/design.md) — why it renders the way it does
- [docs/development.md](docs/development.md) — building, tests and fixtures
