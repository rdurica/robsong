# Domain docs

Architecture Decision Records (ADRs): short markdown files that record **why** option A was chosen over B when the decision is hard to reverse.

## Where

`docs/adr/` at the target repo root (not in the skills pack).

```
docs/adr/
  0001-use-kernel-browser-for-http-tests.md
```

If the folder is missing, proceed silently. `/align` creates files here only for decisions with real trade-offs.

Each file: **Context**, **Decision**, **Consequences**. Filename `NNNN-slug.md` (next number after existing files).

## Conflicts

If a proposal contradicts a file in `docs/adr/`, say so explicitly before continuing.
