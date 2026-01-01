# Issue File Format

Each issue is a Markdown file with YAML front matter:

```markdown
---
number: 123
title: Fix login bug on mobile Safari
labels:
  - bug
  - ios
assignees:
  - alice
  - bob
milestone: v2.0
type: Bug
state: open
state_reason:
synced_at: 2025-12-29T17:00:00Z
---

The body of the issue goes here!
```

## Front Matter Fields

| Field | Type | Description | Editable |
|-------|------|-------------|----------|
| `number` | int/string | Issue number or local ID (T1, T2) | No (managed) |
| `title` | string | Issue title | Yes |
| `labels` | string[] | Label names | Yes |
| `assignees` | string[] | GitHub usernames | Yes |
| `milestone` | string | Milestone name | Yes |
| `type` | string | Issue type (org repos only) | Yes |
| `projects` | string[] | Project names | Yes |
| `state` | string | `open` or `closed` | Via folder |
| `state_reason` | string | `completed` or `not_planned` | Yes |
| `parent` | int | Parent issue number | Yes |
| `blocked_by` | int[] | Blocking issue numbers | Yes |
| `blocks` | int[] | Issues this blocks | Yes |
| `synced_at` | datetime | Last sync time | No (managed) |

## File Naming

Files are named `{number}-{slug}.md` where slug is derived from the title:
- `123-fix-login-bug.md`
- `T1-new-feature.md`

The slug is for readability only, the tool identifies issues by the number prefix.

## Directory Structure

Issues are organized by state:

```
.issues/
├── open/           # Open issues
│   ├── 123-fix-bug.md
│   └── T1-new-feature.md
├── closed/         # Closed issues
│   └── 45-old-bug.md
└── .sync/          # Sync metadata (do not edit)
    └── originals/  # Original versions for conflict detection
```

## Pending Comments

You can queue a comment to be posted when pushing an issue. Create a file named
`{number}.comment.md` in the same directory as the issue:

```bash
# Create a pending comment for issue #42
echo "Updated the acceptance criteria based on PM feedback." > .issues/open/42.comment.md

# The comment will be posted on push
gh-issue-sync push
```

The comment file is automatically deleted after successfully posting. This is
useful for agents or batch workflows that want to leave notes when updating issues.

To skip posting comments during push:

```bash
gh-issue-sync push --no-comments
```
