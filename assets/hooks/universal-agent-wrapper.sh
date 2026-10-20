#!/usr/bin/env bash
# Universal Daneel Agent Wrapper
#
# Enables ANY arbitrary AI agent CLI (e.g. codex, aider, custom scripts) to
# benefit from Daneel's passive capture, ambient telemetry, session compilation,
# and Dream Cycle consolidation.
#
# Usage:
#   ./universal-agent-wrapper.sh <agent-name> <command> [args...]
#
# Examples:
#   ./universal-agent-wrapper.sh codex codex "Refactor internal/auth"
#   ./universal-agent-wrapper.sh aider aider --model gpt-4o
#   ./universal-agent-wrapper.sh custom-agent python run_agent.py

set -euo pipefail

AGENT_NAME="${1:-custom-agent}"
shift

if [ "$#" -eq 0 ]; then
  echo "Usage: $0 <agent-name> <command> [args...]"
  exit 1
fi

SESSION_ID="sess-$(date +%s)-$RANDOM"
CWD="$(pwd)"

# 1. Provide handoff context from recent sessions in this repository
daneel hook handoff --cwd "$CWD" || true

# 2. Emit SessionStart event
START_PAYLOAD=$(cat <<EOF
{
  "sessionId": "$SESSION_ID",
  "agent": "$AGENT_NAME",
  "cwd": "$CWD",
  "command": "$*"
}
EOF
)
echo "$START_PAYLOAD" | daneel hook capture SessionStart --agent "$AGENT_NAME" --session "$SESSION_ID" --cwd "$CWD" || true

# 3. Execute the actual agent command
EXIT_CODE=0
"$@" || EXIT_CODE=$?

# 4. Emit SessionEnd event (triggers automatic raw-to-inbox markdown compilation)
END_PAYLOAD=$(cat <<EOF
{
  "sessionId": "$SESSION_ID",
  "agent": "$AGENT_NAME",
  "cwd": "$CWD",
  "exit_code": $EXIT_CODE
}
EOF
)
echo "$END_PAYLOAD" | daneel hook capture SessionEnd --agent "$AGENT_NAME" --session "$SESSION_ID" --cwd "$CWD" || true

exit $EXIT_CODE
