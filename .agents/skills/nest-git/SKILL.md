---
name: nest-git
description: Prepare Nest commits and pull requests using repository conventions. Use when staging changes, composing commit messages, or drafting pull requests.
---

# Nest Git

Follow the standing Git rules in AGENTS.md.

- Review the staged diff and check that it matches the approved scope.
- Keep commit subjects concise and within 50 characters.
- Explain what changed and why in the body. Wrap lines at 72 columns.
- Pass multiline commit messages through a quoted heredoc with `git commit -S -F -`.
- Write the pull request title and plain-text description as the intended squash commit message.
