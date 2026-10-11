| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-11 | appliance-ships-ze-kernel | `runDocker`, `docker run --rm` of the kernel container | Unprobed (round-8 NOTE 2): a stop SIGKILLs the Docker CLI, which cannot forward it, so the container probably keeps compiling and holds the ze-kernel-build volumes the next build mounts | not fixed: name the container and `docker stop` it on a stop, proven against a real daemon |
