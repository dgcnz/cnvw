# Development

```sh
go build -o cnvw .
go test ./...
```

Go 1.25.8 or newer. The Charm v2 libraries need a Go newer than some systems
ship, so `mise.toml` pins one; mise is a convenience for building this repo,
not a dependency of the tool. With it, prefix the above with `mise exec --`.

## Fixtures

`testdata/` exists for `go test`, which cannot run without it, rather than for
reading; `docs/*.txt` holds rendered output, which is what a reader wants. The
fixtures are small — 40K in total — and each has a different shape, to catch a
different class of bug:

| Fixture | Exercises |
|---|---|
| `sample.canvas` | every node type, every edge side combination, both colour encodings |
| `grouped.canvas` | nested groups, mixed routing, wide and short nodes |
| `labeled-tree.canvas` | 37 edges, 32 labelled, almost all bottom-to-top |

The last two derive from real hand-made canvases. They keep the geometry,
colours and edge topology that caught most of the bugs here, and carry filler
text: the renderer is being tested on the layout, not the prose.

Regenerate the rendered frames after a change to the renderer:

```sh
for f in sample grouped labeled-tree; do
  ./cnvw --dump 100x24 testdata/$f.canvas | sed -e 's/\x1b\[[0-9;]*m//g' > docs/$f.txt
done
```

## Benchmarks

The stress fixture is generated rather than checked in:

```sh
python3 testdata/gen_large.py 2000 > testdata/large.canvas
go test ./internal/render/ -run XXX -bench Large
```

Render cost is fixed per frame and scales with the terminal size rather than
the canvas: about 4ms for a 200x50 viewport whether the document holds nine
nodes or two thousand, since projecting 2000 nodes is only 0.2ms of that. If
that ever matters, the win is reusing the cell buffer between frames instead of
allocating one per render.
