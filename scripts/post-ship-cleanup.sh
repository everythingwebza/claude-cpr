#!/usr/bin/env bash
# post-ship-cleanup.sh — retire the legacy Python claude-projects script
# once the Go cpr binary has been in use for a while.
#
# Safe to run anytime. Performs 5 checks; only takes destructive action
# (rm of the Python fallback) when ALL pass. Prints a clear status report
# either way.
#
# After this runs successfully and you trust the result, you can delete
# this script — it's a one-time-after-shipping helper, not part of normal
# operation.

set -u  # not -e: we want to continue checking even if one check fails

GO_BIN="$HOME/.local/bin/cpr"
STATE_FILE="$HOME/.claude/.cpr-state.json"
PYTHON_FALLBACK="$HOME/scripts/claude-projects"
BASHRC="$HOME/.bashrc"

passed=()
failed=()

# 1. Go binary exists and is executable
if [[ -x "$GO_BIN" ]]; then
    passed+=("Go binary at $GO_BIN exists and is executable")
else
    failed+=("Go binary missing or not executable at $GO_BIN")
fi

# 2. State file modified within the last 7 days
if [[ -f "$STATE_FILE" ]]; then
    now_s=$(date +%s)
    mtime_s=$(stat -c %Y "$STATE_FILE" 2>/dev/null || stat -f %m "$STATE_FILE")
    age_days=$(( (now_s - mtime_s) / 86400 ))
    if (( age_days <= 7 )); then
        passed+=("State file modified ${age_days} day(s) ago — Go binary is in active use")
    else
        failed+=("State file last modified ${age_days} days ago — has the Go binary been used recently?")
    fi
else
    failed+=("State file $STATE_FILE doesn't exist — has the Go binary been launched at all?")
fi

# 3. ~/.bashrc has no 'alias cpr=' line
if grep -qE "^[[:space:]]*alias[[:space:]]+cpr[[:space:]]*=" "$BASHRC" 2>/dev/null; then
    failed+=("$BASHRC still has an 'alias cpr=' line — alias should have been removed at v0.1.0 ship")
else
    passed+=("$BASHRC has no stale 'alias cpr=' line")
fi

# 4. Confirm `cpr` resolves to the Go binary
resolved=$(command -v cpr 2>/dev/null || true)
if [[ "$resolved" == "$GO_BIN" ]]; then
    passed+=("'cpr' resolves to $resolved (the Go binary)")
elif [[ -n "$resolved" ]]; then
    failed+=("'cpr' resolves to '$resolved', not the expected '$GO_BIN'")
else
    failed+=("'cpr' is not on \$PATH at all")
fi

# 5. The Python fallback still exists (otherwise nothing to clean up)
if [[ -f "$PYTHON_FALLBACK" ]]; then
    passed+=("Python fallback present at $PYTHON_FALLBACK — ready to retire")
else
    passed+=("Python fallback already gone at $PYTHON_FALLBACK — nothing to retire")
fi

# Report
echo "=== cpr post-ship cleanup ==="
echo
echo "Checks passed (${#passed[@]}):"
for line in "${passed[@]}"; do printf '  ✓ %s\n' "$line"; done

if (( ${#failed[@]} > 0 )); then
    echo
    echo "Problems found (${#failed[@]}) — NOT cleaning up:"
    for line in "${failed[@]}"; do printf '  ✗ %s\n' "$line"; done
    echo
    echo "Fix the issues above and re-run, or restore the alias if you've"
    echo "switched back to the Python version:"
    echo "    echo \"alias cpr='claude-projects'\" >> ~/.bashrc"
    exit 1
fi

# All checks passed → retire the Python fallback if it's still around
echo
if [[ -f "$PYTHON_FALLBACK" ]]; then
    echo "All checks passed. Retiring Python fallback:"
    rm -v "$PYTHON_FALLBACK"
    echo
    echo "Done. cpr is now Go-only. You can delete this script."
else
    echo "All checks passed and Python fallback is already gone."
    echo "Nothing to do. You can delete this script."
fi
