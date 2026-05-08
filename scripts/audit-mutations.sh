#!/usr/bin/env bash
# audit-mutations.sh — find genqlient-generated input types that risk
# null serialization rejection by GitHub (see docs/design/genqlient-quirks.md).
#
# Output (markdown table) is written to stdout. Exit code 1 when any
# input type is missing a hand-written workaround, so this script can
# gate CI.
#
# Heuristics:
#   - Pattern 1: `[]T \`json:"X"\`` (no omitempty) on an input type whose
#     name ends in `Input` and is referenced from cmd/. Flagged if the
#     queries package does NOT define a `New<TypeName>` constructor.
#   - Pattern 2: oneOf-style inputs (currently `ProjectV2FieldValue`)
#     listed in `ONEOF_TYPES` below, flagged if the queries package
#     does NOT define a `MarshalJSON` method on the type. The list is
#     manually curated because regular Input structs with many optional
#     `*T` fields are NOT oneOf and GitHub accepts them — heuristic
#     pointer-counting produces false positives.
#
# Usage:
#   scripts/audit-mutations.sh           # report
#   scripts/audit-mutations.sh --check   # report + exit 1 on findings (CI / pre-commit)

set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
GENQLIENT="$ROOT/internal/github/queries/genqlient.go"
WORKAROUND_FILES=(
  "$ROOT/internal/github/queries/input_constructors.go"
)

CHECK_MODE=0
if [ "${1:-}" = "--check" ]; then
  CHECK_MODE=1
fi

if [ ! -f "$GENQLIENT" ]; then
  echo "ERROR: $GENQLIENT not found. Run \`go generate ./...\` first." >&2
  exit 2
fi

# Type names that the cmd/ tree actually invokes. We only flag findings
# on input types reachable from a cmd; unused generated types are noise.
USED_INPUTS=$(
  grep -rh -oE 'queries\.(New)?[A-Z][A-Za-z0-9]*Input' "$ROOT/cmd/" 2>/dev/null |
    sed -E 's/queries\.(New)?//' |
    sort -u
)

# Workaround inventory: which types have a constructor / MarshalJSON.
WORKAROUND_BLOB=$(cat "${WORKAROUND_FILES[@]}" 2>/dev/null || true)

has_constructor() {
  echo "$WORKAROUND_BLOB" | grep -q "func New$1("
}

has_marshaljson() {
  # matches `func (v *Type) MarshalJSON()` or `func (Type) MarshalJSON()`
  echo "$WORKAROUND_BLOB" | grep -qE "func \([^)]*\*?$1\) MarshalJSON\("
}

# Extract a single struct block from genqlient.go for `awk` scanning.
struct_body() {
  awk -v t="$1" '
		$0 ~ "^type " t " struct {" { in_block=1; next }
		in_block && $0 == "}" { exit }
		in_block { print }
	' "$GENQLIENT"
}

list_pattern1_fields() {
  # `[]T \`json:"X"\`` without omitempty on the same line.
  # grep is allowed to return 1 (no match); we want awk to receive empty
  # input rather than the whole helper exiting under set -eo pipefail.
  # shellcheck disable=SC2016 # backticks are literal in genqlient struct tags
  struct_body "$1" |
    { grep -E '\[\][A-Za-z][A-Za-z0-9_*\.]* `json:"[a-zA-Z_]+"`$' || true; } |
    awk '{print $1}'
}

findings_p1=()
findings_p2=()

for type_name in $USED_INPUTS; do
  # Pattern 1
  p1=$(list_pattern1_fields "$type_name")
  if [ -n "$p1" ]; then
    if ! has_constructor "$type_name"; then
      fields=$(echo "$p1" | tr '\n' ',' | sed 's/,$//')
      findings_p1+=("$type_name|$fields")
    fi
  fi
done

# Manually curated list of oneOf-style inputs. New oneOf types
# discovered during mutation audits go here. The heuristic
# alternative (count nullable pointers) produces false positives on
# regular Input structs with many optional fields.
ONEOF_TYPES=(ProjectV2FieldValue)

for type_name in "${ONEOF_TYPES[@]}"; do
  if ! has_marshaljson "$type_name"; then
    findings_p2+=("$type_name")
  fi
done

# Reporting
echo "# genqlient mutation audit"
echo
echo "Generated $(date -u +%Y-%m-%dT%H:%M:%SZ) — see docs/design/genqlient-quirks.md"
echo

if [ ${#findings_p1[@]} -eq 0 ]; then
  echo "## Pattern 1 (nullable list null) — ✅ all covered"
else
  echo "## Pattern 1 (nullable list null) — ❌ ${#findings_p1[@]} type(s) missing constructor"
  echo
  echo "| Input type | Affected fields |"
  echo "| --- | --- |"
  for row in "${findings_p1[@]}"; do
    t="${row%%|*}"
    f="${row#*|}"
    echo "| $t | $f |"
  done
fi

echo

if [ ${#findings_p2[@]} -eq 0 ]; then
  echo "## Pattern 2 (oneOf nested null) — ✅ all covered"
else
  echo "## Pattern 2 (oneOf nested null) — ❌ ${#findings_p2[@]} type(s) missing MarshalJSON"
  echo
  echo "| Input type |"
  echo "| --- |"
  for t in "${findings_p2[@]}"; do
    echo "| $t |"
  done
fi

total=$((${#findings_p1[@]} + ${#findings_p2[@]}))
if [ "$CHECK_MODE" -eq 1 ] && [ "$total" -gt 0 ]; then
  echo
  echo "ERROR: $total mutation input type(s) need workaround. See docs/design/genqlient-quirks.md." >&2
  exit 1
fi
