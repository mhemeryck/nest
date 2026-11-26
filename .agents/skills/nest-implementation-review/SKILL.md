---
name: nest-implementation-review
description: Walk through a Nest implementation with the user using an architecture diagram and call graphs grounded in the code. Use for collaborative implementation reviews and code walkthroughs.
---

# Nest Implementation Review

Read the implemented change and the agreed plan.
Flag material differences that affect the review.

1. Show a Mermaid diagram of the implemented architecture.
   Label component responsibilities, state ownership, and data flow where relevant.
2. Choose entry points that exercise the change, such as a CLI command or an event handler.
   Trace actual calls and show the relevant paths as Mermaid call graphs.
   Label asynchronous message or event handoffs.
   Include branches and failure paths that change the outcome.
3. Link the entry points and key functions to their source locations.
   Use the graphs to walk through the code with the user and identify small refinements.
