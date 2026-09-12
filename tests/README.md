# Tests

- `go test ./...` — unit tests (registry, session, drivers, shell quoting).
- `tests/acceptance.sh` (to come) — plays the acceptance criteria A1–A13 of
  the specification inside a Hyprland VM. Never run it on a personal machine:
  it creates outputs, launches compositors and kills processes.
