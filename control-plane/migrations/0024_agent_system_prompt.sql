-- Custom non-empty AGENT system-prompt suffix, set from the console
-- (Settings → Agent instructions (custom)). Appended after the embedded
-- platform briefing (internal/agentprompt/prompt.md — which stays read-only,
-- compiled into the binaries) with a delimiter, and rendered with the same
-- per-sandbox placeholders ({{APP_DIR}}, {{PORT}}, {{HEALTH_PATH}}, {{LOCAL_URL}}).
-- Empty string = disabled (platform briefing alone). Applied to NEXT tasks
-- only — tasks already running keep the briefing they were started with.
-- Non-secret — safe to show any operator.

ALTER TABLE instance_settings ADD COLUMN agent_system_prompt TEXT NOT NULL DEFAULT '';