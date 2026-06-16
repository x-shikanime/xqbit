# xqbit

Extra tooling for qBittorrent — CLI utilities for managing torrents.

**Language:** Go

## Structure

- `cmd/` — CLI entrypoints
  - `cmd/qbittorrent-cleanup/` — Manages errored torrents in qBittorrent

## Build / Test Commands

- Run checks: `go test ./...`
- Run qbittorrent-cleanup: `go run ./cmd/qbittorrent-cleanup --help`

## Commit Style

- Plain-text capitalized title, no conventional-commit prefix
- Body with labels: `Design:`, `Related:`, `Closes #`
- Keep Markdown lines wrapped at 80 columns and run `nix fmt` before shipping

## Stack

- 1 commit == 1 PR via ghstack (1 commit is 1 logical atomic change)
- Split work into stacked PRs to keep each PR small and reviewable
- To update a PR: edit files, then `jj squash` (or `git commit --amend`) into the
  **target commit** of the stack — the one that PR represents
- Resubmit with `ghstack` after squashing
- `ghstack land` on the head PR to land the entire stack
- Never `gh pr merge` (creates poisoned commits)
- Never force-push ghstack branches

## Protect `main`

- Require 1 approving review
- Require linear history (no merge commits)
- Require signed commits
- Squash+rebase merge only

*Always use worktrees when making changes. Test with `go test ./...` before
submitting.*
