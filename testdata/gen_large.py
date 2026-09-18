#!/usr/bin/env python3
"""Generate a large .canvas file for checking culling and render cost.

Usage: python3 testdata/gen_large.py [nodes] > testdata/large.canvas
"""
import json
import random
import sys

n = int(sys.argv[1]) if len(sys.argv) > 1 else 2000
random.seed(7)

COLS = 50
W, H = 250, 120
GAP_X, GAP_Y = 340, 220

words = "ingest normalize enrich dedupe partition compact archive replay audit".split()

nodes, edges = [], []
for i in range(n):
    col, row = i % COLS, i // COLS
    nodes.append({
        "id": f"n{i}",
        "type": "text",
        "x": col * GAP_X,
        "y": row * GAP_Y,
        "width": W,
        "height": H,
        "color": str(random.randint(1, 6)),
        "text": f"## {random.choice(words)} {i}\n\nStage {i} of the pipeline, **{random.choice(words)}** step.",
    })

# Chain each node to its right-hand neighbour, plus some longer hops so the
# router sees edges whose endpoints are far apart.
for i in range(n - 1):
    if (i + 1) % COLS:
        edges.append({"id": f"e{i}", "fromNode": f"n{i}", "fromSide": "right",
                      "toNode": f"n{i+1}", "toSide": "left"})
for i in range(0, n - COLS, 37):
    edges.append({"id": f"h{i}", "fromNode": f"n{i}", "fromSide": "bottom",
                  "toNode": f"n{i+COLS}", "toSide": "top", "label": "next"})

json.dump({"nodes": nodes, "edges": edges}, sys.stdout)
