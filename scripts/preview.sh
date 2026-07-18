#!/usr/bin/env bash
#
# preview.sh — the inner loop. Run an animation's preview program live in this
# terminal so you can watch it. The program owns the loop; Ctrl-C to quit.
#
# Usage:
#   scripts/preview.sh [run-command...]     # default: go run ./cmd/preview
#
# For a standalone animation, cmd/preview is the scaffold in preview.go.tmpl.
# For a fresco variant, use the preview program fresco's `new-variant` skill has
# you write — it selects your variant and sweeps LumRange, which is what tuning
# needs. (`./cmd/fresco-demo` only cycles the whole shipped roster on a timer: a
# final look at the merged result, not a per-variant tuning knob.)
set -euo pipefail

if [[ $# -eq 0 ]]; then
	set -- go run ./cmd/preview
fi

echo "→ $*   (Ctrl-C to quit)"
exec "$@"
