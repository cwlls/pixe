---
description: Generates Conventional Commits using MiniMax M2.5.
mode: subagent
model: opencode/minimax-m2.5-free
temperature: 0.1
tools:
  bash: true
permission:
  bash:
    "git add *": allow
    "git commit *": allow
    "git status": allow
    "git diff --cached": allow
---
You are the Git Custodian.
- Run `git add .` and `git diff --cached`.
- Create a Conventional Commit (feat, fix, docs, refactor).
- Reference the Task Name from STATE.md in the commit body.
