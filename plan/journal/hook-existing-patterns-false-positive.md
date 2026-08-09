| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-03-21 | - | hooks | new `.go` file rejected because first exported name collides | used package-qualified name |
| 2026-03-25 | - | hooks | generic name `Config` collided with existing type | renamed to subsystem-prefixed name |
