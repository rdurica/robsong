## Agent skills

Issue tracker: GitHub. See `docs/agents/config/issue-tracker.md`.
Domain docs: `docs/adr/`. See `docs/agents/config/domain.md`.
Workflow defaults: `docs/agents/config/workflow.md` (branch-owner, push, language, work types).
Pipeline: `/align` → `/analyze` → `/implement` → `/verify` (functional → code review [→ ux] → finalize).
`/implement` (Opus) orchestrates `test-writer` agents → parent test commit → `implementer` agents → automatic `/verify` with reviewer and `check-runner` agents (Sonnet/Haiku); see the skill for detailed rules.
Skills: the shared pack installed in `~/.claude/`; `/setup` updates project configuration, not the pack. Repo-specific additions belong in `docs/agents/config/`.
