# Docker in agent sessions

Many sessions share one Docker daemon on this machine. An agent MUST NOT run a
host-wide destructive Docker command: `docker container prune`, `docker system
prune`, `docker image prune`, `docker volume prune`, `docker network prune`,
`docker rm` or `docker rmi` of anything it did not create in the same run.
Remove only the containers, networks and images the agent's own run created,
by name. A stopped container may be another session's evidence.

Origin: on 2026-10-08 an agent ran `docker container prune -f` while debugging
an interop scenario and removed every stopped container on the host.
