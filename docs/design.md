# Design

How cnvw decides what to draw, and why. Each of these was a bug before it was a
rule.

**Zoom is anchored to text, not to pixels.** One terminal cell is 8x16 canvas
pixels at zoom 1.0, roughly Obsidian's own character cell. Scaling by a cell's
visual 2:1 shape instead would squash a default 250x60 node into two rows,
leaving nothing between its borders. See `internal/geom/camera.go`.

**Both corners of a node are rounded independently** and the size taken as
their difference, rather than rounding position and size separately. Abutting
nodes stay flush and boxes do not twitch by a cell while panning.

**The opening view is legible before it is complete.** A purely geometric fit
answers "how do I get everything on screen", which on a large canvas shrinks
every node past the point where it can say anything: an accurate picture of the
layout carrying no information. `internal/geom/legible.go` answers the other
question, "how close do I have to be for the cards to mean something", and the
opening view takes whichever is closer, then slides the viewport back over the
canvas so no screen space is spent beyond its edge. `f` is always the way back
out to the whole thing.

The threshold is the median node, not every node: requiring all of them to be
legible would let one narrow outlier zoom the canvas in to nothing.

**Detail comes from the node's size on screen, not from the zoom**, so a large
node stays readable when a small one beside it has already collapsed:

| Tier | Size | Drawn as |
|---|---|---|
| full | 12x3 cells and up | border, wrapped body |
| title | 6x3 | border, one truncated line |
| mini | 3x2 | a filled bar with the title |
| block | 1x1 | a solid block of color |

Groups never collapse to a block, since filling their rectangle would bury
their contents.

**Edges route orthogonally.** Each end steps clear of its border, then the two
ends are joined by an elbow or by a crossbar. The crossbar has to sit beyond
both ends when they leave through the same side, or the path doubles back into
its own destination. Crossing lines merge by connection mask into `├ ┼ ┴`
rather than one overwriting the other. See `internal/render/edge.go`.

**Edge labels go beside a vertical run and inside a horizontal one.** A canvas
laid out top to bottom connects its nodes bottom-to-top, which leaves the long
run on the vertical axis, where text written one character per row is
unreadable. Labels are placed in a second pass once every line is down, they
never take cells a node is about to cover, and placement is all or nothing. A
label that does not fit is dropped: at low zoom that thins them out on its own,
which beats a row of half-overwritten words.

**Layers are opaque**, which fixes the draw order: groups, then edges, then the
nodes that cover the edge stubs running underneath them. That happens to be how
Obsidian stacks them too.

**LaTeX math and wiki links are resolved before anything renders them.**
Glamour is goldmark-based and knows neither, so a note written with
`$J_x^\top J_x$` and `[[100 Reference notes/101 Literature/2ndMatch|2ndMatch]]`
reaches the screen with its dollar signs, backslashes and whole vault path
intact. `internal/mathtex` maps math to the nearest plain Unicode
(`Jₓᵀ Jₓ`) and keeps the LaTeX wherever Unicode has no equivalent, since half a
translation still reads and a wrong symbol does not. Wiki links show their
alias, or the last path segment when there is none. Both apply to canvas boxes
and to the reader pane.

**Markdown inside a node is formatted, not rendered.** `internal/mdline` strips
the syntax and keeps the emphasis, so a heading reads as bold text rather than
`## Heading`. Glamour is reserved for the reader pane, where the width justifies
it; inside a twelve-cell box its margins and cost do not.

## Layout

```
main.go              flags, load, run
internal/canvas/     JSON Canvas parsing and model
internal/geom/       camera, projection, detail tiers, spatial jumps
internal/render/     node boxes, edge routing, themes, compositing
internal/mathtex/    LaTeX math spans to plain Unicode
internal/mdline/     Markdown to short styled lines
internal/app/        event loop, keys, search, reader pane
```

**There is one way to do each thing.** Panning had three tiers at one point —
a step, a half screen and a full screen — which are `less` idioms that assume a
single axis. With two axes and a zoom, distance is covered by zooming out, so
the paging keys went and the step grew to a quarter of the viewport. Reset-zoom
went too, since `f` and `z` both land somewhere more useful than an arbitrary
1.0, and "first/last node" went because reading order is a weak idea on a
canvas.

Movement is on the arrows rather than on `hjkl`. A canvas is not text and the
modal habit buys nothing here, whereas an arrow needs no explaining. The one
concession is that `H` `J` `K` `L` still jump: a shifted arrow is widely but
not universally reported, and on a terminal that swallows it the jump would
otherwise have no key at all. They are a fallback, not the documented path.

## Not included

Editing, dragging, resizing, saving, undo, and full Obsidian compatibility.
Group background images are parsed but not drawn: a terminal has nowhere to put
them without an image protocol.

Known gaps, in rough order of how much they would add:

- **Nothing shows what a node connects to.** Selecting a node does not
  highlight its edges or list their labels, so a label that could not be placed
  is invisible at that zoom. Jumping the camera along edges is deliberately not
  the answer: you read a cluster of cards at once, and a camera that moves per
  edge just has to be readjusted.
- **No minimap**, which is the usual answer to losing your place on a large
  canvas. Panning is unclamped, so it is possible to pan into empty space; `f`
  is the way back.
- **Search is plain substring**, with no regex and no list of matches.
