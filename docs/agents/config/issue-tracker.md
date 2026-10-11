# Issue tracker: GitHub

Issues and analyses live as GitHub issues. Use the `gh` CLI.

Also read `docs/agents/config/workflow.md` for branch-owner and push defaults.

## Which GitHub repo

- **Single repo:** infer from `git remote -v` in the clone (cwd).
- **Monorepo** (`## Monorepo` in `workflow.md`, or nested git repos / submodules): **always** the **container-root** remote — never a delivery-root (`backend`, `frontend`, …).

Resolve container-root `owner/repo` from `git -C <container-root> remote get-url origin` (or `remote:` on `container-root` in `workflow.md` if set). Pass it on every `gh issue` call:

```bash
gh issue create -R <owner/container-repo> ...
gh issue view <N> -R <owner/container-repo> ...
gh issue edit <N> -R <owner/container-repo> ...
```

Do not invent a delivery-root issue tracker. Agent-owned PRs still open in delivery roots (`/verify`); human-owned PRs remain with the user; only issues/analyses live on the monorepo.

## Conventions

Always add `-R <owner/repo>` in a monorepo (container-root remote — see above).

- **Create:** `gh issue create [-R …] --title "..." --body-file <file> --label "..."`
- **Read:** `gh issue view <number> [-R …] --comments`
- **Edit body:** `gh issue edit <number> [-R …] --body-file <file>`
- **List:** `gh issue list [-R …] --state open --json number,title,body,labels`
- **Comment:** `gh issue comment <number> [-R …] --body-file <file>`
- **Labels:** `gh issue edit <number> [-R …] --add-label "..."` / `--remove-label "..."`
- **Close:** `gh issue close <number> [-R …] --comment "..."`

## Labels

| Label | Set by | Meaning |
|-------|--------|---------|
| `analysis` | `/analyze` | This issue is an analysis |
| `ready-for-agent` | human, after validating the analysis | Auto-trigger may start `/implement`. Agent never adds this. Happy path: user runs `/implement` after review |
| `in-progress` | `/implement` | Agent is working |
| `needs-attention` | `/verify` (fail) or `/implement` (blocked by open FAQ) | Agent stopped and cannot continue without a human decision or fix. **Replaces** `in-progress` — never both |
| `ready-to-review` | `/verify` (green) | Verified delivery ready for human review; publishing-enabled agent-owned delivery also has a ready PR |

`/implement` drops `ready-for-agent` and `needs-attention` when it sets `in-progress`. A green `/verify` removes pipeline working labels and sets `ready-to-review`. Preserve unrelated labels (team, priority, kind). Keep `analysis` when present; bug fast-path tickets need not have it. Human-owned delivery does not require a PR. The completion comment records verified SHA, local gates, push and CI outcome, and PR URLs when applicable.

User invoked `/implement` → run it (do not require `ready-for-agent`). Stop if `ready-to-review`. `needs-attention` does **not** stop it — that is the resume path.

Auto-start without the user → only if `ready-for-agent` or already `in-progress`. **Never** when `needs-attention` is set, even with `ready-for-agent` — a human has to act first.

Create a label on first use if it is missing (`gh label create <name> [-R …]`; ignore "already exists").

The normal workflow starts locally when the user invokes `/implement`; the labels below support optional automation, not a requirement to claim work.

**Lifecycle:** `analysis` → user reviews, then `/implement` (optional `ready-for-agent` for auto-start) → `in-progress` → `ready-to-review`. Verify fail → `needs-attention` and a comment on this issue; agent-owned delivery may have a draft PR only when push and PR operations are allowed. Remaining work blocked by open `## FAQ` → `needs-attention`. Session died → leave `in-progress` (resumable); re-run `/implement`.

## Delivery section

After `/analyze`, prepend to issue body:

```markdown
## Delivery

- Ticket: [AB#4821](https://dev.azure.com/org/project/_workitems/edit/4821)
- Kind: feature | bug | chore
- Branch: `feature/4821-group-pricing`
```

`Ticket` is the external tracker item when there is one (clickable link), otherwise this issue: `Ticket: [#<N>](<issue url>)`. `Branch` reuses that same number as `feature/<ticket>-<short-slug>`. Commits and PRs still reference the analysis issue as `#N` — that is the number GitHub autolinks.

## Skill operations

### `/analyze` — publish analysis

Ensure the `analysis` label exists (`gh label create analysis --description "Published analysis"`; ignore "already exists").

Write the complete analysis to a temporary file, then pass it with `--body-file`. After creating the issue, add Delivery to that file and update the issue body. Preserve real newlines and literal contract values.

```bash
# Add -R <owner/container-repo> when ## Monorepo
gh issue create -R <owner/repo> --title "Analysis: ..." --body-file <analysis-file> --label analysis
gh issue edit <number> -R <owner/repo> --body-file <analysis-file-with-delivery>
```

Do not add `ready-for-agent`.

### `/implement` — fetch, status, comment

Same `-R` rule as `/analyze` (container-root in monorepo). Fetch `#N` by number.

| Operation | Command |
|-----------|---------|
| Fetch analysis/ticket | `gh issue view <N> [-R …] --comments` |
| Set in-progress | `gh issue edit <N> [-R …] --remove-label ready-for-agent --remove-label needs-attention --add-label in-progress` |
| Blocked by open FAQ | `gh issue edit <N> [-R …] --remove-label in-progress --add-label needs-attention` |
| Update Acceptance checkboxes | `gh issue edit <N> [-R …] --body-file <file>` (checked items in `## Acceptance`) |
| Comment | `gh issue comment <N> [-R …] --body-file <file>` (brief interruption/blocker note when needed — not per part) |

Do not add `ready-for-agent`.

### `/verify` — comment on the analysis only (never on the PR)

| Operation | Command |
|-----------|---------|
| Green | `gh issue edit <N> [-R …] --remove-label in-progress --remove-label needs-attention --remove-label ready-for-agent --add-label ready-to-review` then `gh issue comment` (verified SHA, local gates, push/CI outcome, PR URLs when applicable). Preserve unrelated labels |
| Fail | `gh issue edit <N> [-R …] --remove-label in-progress --add-label needs-attention` then `gh issue comment` (what failed, draft PR URLs when applicable). Human-owned never manages PRs; agent-owned draft PR delivery follows push policy and session restrictions |

## When a skill says "publish to the issue tracker"

Create a GitHub issue using the operations above (monorepo → container-root `-R`).

## When a skill says "fetch the relevant ticket"

Run `gh issue view <number> [-R …] --comments`.
