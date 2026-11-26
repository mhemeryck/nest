---
name: nest-markdown
description: Write and edit Nest Markdown with the repository's conventions for prose, design documents, and formatting. Use when creating or revising Markdown in Nest.
---

# Nest Markdown

- Prefer fragments for headings, comments, and scan-oriented lists.
  Use full sentences for narrative or when precision requires them.
- Put one sentence per source line in Markdown prose.
- Write design documents in ASD-STE100 Simplified Technical English.
  Use active voice, consistent terms, and one idea per sentence.
- Run the repository Markdown formatter after editing.

```nu
devenv tasks run dev:format
```
