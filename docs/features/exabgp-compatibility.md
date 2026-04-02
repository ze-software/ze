# ExaBGP Compatibility

<!-- source: cmd/ze/exabgp/main.go -- ze exabgp subcommands -->
<!-- source: cmd/ze/exabgp/main_sdk.go -- SDK/TLS connect-back mode -->
<!-- source: internal/exabgp/migration/migrate.go -- ExaBGP config migration -->

- Automatic detection and migration of ExaBGP configuration files
- `ze exabgp plugin` runs ExaBGP processes with ze as the BGP engine
- Two modes: standalone (stdin/stdout) for development, TLS connect-back when launched by engine
- Bidirectional translation: ze JSON events to ExaBGP JSON, ExaBGP text commands to ze commands
- Forward-barrier flush injected after route commands for ordering guarantees
- `ze exabgp migrate` converts ExaBGP configs to ze format
- `ze exabgp migrate --env` converts ExaBGP INI environment files to ze config
