# Spec: Environment Configuration Block

## SOURCE FILES (read before implementation)

```
┌─────────────────────────────────────────────────────────────────┐
│  Read these source files before implementing:                   │
│                                                                 │
│  1. internal/config/environment.go (Environment struct, loading)     │
│  2. internal/config/environment_test.go (test patterns)              │
│  3. internal/config/schema.go (config parsing patterns)              │
│  4. internal/config/parser.go (main config parser)                   │
│  5. docs/architecture/config/SYNTAX.md (syntax reference)           │
│                                                                 │
│  ExaBGP Reference:                                              │
│  - /Users/thomas/Code/github.com/exa-networks/exabgp/main/      │
│    src/exabgp/environment/config.py (INI file loading)          │
│                                                                 │
│  NOTE: ExaBGP uses SEPARATE .env INI file, NOT config block.    │
│  This feature is ZeBGP-specific enhancement.                    │
│                                                                 │
│  ON COMPLETION: Update docs/architecture/config/SYNTAX.md           │
└─────────────────────────────────────────────────────────────────┘
```

## Task

Add `environment` block support to ZeBGP configuration files, allowing users to set environment variables from within the config file instead of shell.

## Current State

- Tests: `make test` PASS, `make lint` PASS
- API functional: 14/14 passed (100%)

## Problem Analysis

### Current Flow

1. `LoadEnvironment()` called in `internal/config/loader.go`
2. Sets defaults via `loadDefaults()`
3. Reads OS environment via `loadFromEnv()`
4. **Silent ignore** on parse errors (e.g., `ze.bgp.tcp.port=abc` → uses default)
5. No config file support

### User Goal

Set environment variables in config file using nested blocks:

```
environment {
    log {
        level DEBUG;
    }
    tcp {
        port 1179;
    }
    api {
        encoder json;
    }
}
```

### ExaBGP Approach (for reference)

ExaBGP uses a **separate INI file** (`/etc/exabgp/exabgp.env`):

```ini
[exabgp.log]
level = DEBUG

[exabgp.tcp]
port = 1179
```

**NOT** embedded in main config. ZeBGP will enhance this by supporting both:
1. Environment block in main config file
2. OS environment variables (higher priority)

### Priority Order (ZeBGP)

```
OS env vars (ze.bgp.x.y) > OS env vars (zebgp_x_y) > config file > defaults
```

## Goal Achievement

```
🎯 User's actual goal: Set environment from config file

| Check | Status |
|-------|--------|
| Config syntax works? | ❌ Not implemented |
| Parser handles it? | ❌ No environment block parsing |
| Environment applied? | ❌ No config → Environment bridge |
| Invalid values rejected? | ❌ Currently silent ignore |

Plan achieves goal: YES (after implementation)
```

## Embedded Rules

- TDD: test must fail before impl
- Verify: `make test && make lint` before done
- No RFC involved (internal feature)

## Documentation Impact

### New Documentation (create)

- [ ] `docs/architecture/config/ENVIRONMENT_BLOCK.md` - New feature doc

### Updates (modify existing)

- [ ] `docs/architecture/config/SYNTAX.md` - add environment block to section list
- [ ] `docs/architecture/config/ENVIRONMENT.md` - add config block as source, update priority
- [ ] `.claude/INDEX.md` - add ENVIRONMENT_BLOCK.md to navigation
- [ ] `docs/plan/CLAUDE_CONTINUATION.md` - update after impl

## Design

### Config Syntax

```
environment {
    log {
        level DEBUG;
        destination /var/log/ze.bgp.log;
        short true;
    }
    tcp {
        port 1179;
        attempts 3;
    }
    api {
        encoder json;
        respawn true;
        cli true;
    }
    daemon {
        user zebgp;
        daemonize true;
        drop true;
    }
    bgp {
        passive true;
        openwait 120;
    }
    cache {
        attributes true;
    }
    reactor {
        speed 1.0;
    }
    debug {
        timing true;
        memory false;
    }
}
```

**Nested block structure** matches ZeBGP's JUNOS-like config style.
Maps directly to Environment struct sections (LogEnv, TCPEnv, etc.).

### Constraints

1. **Position:** Environment block can appear anywhere but is processed first.
   Recommended at top of file for clarity, but not enforced.

2. **Single block:** Only ONE environment block allowed. Multiple blocks = parse error.

3. **Strict validation - REFUSE TO START on invalid values:**
   - Unknown section → error, refuse to start
   - Unknown option → error, refuse to start
   - Type parse failure (e.g., `port abc`) → error, refuse to start
   - Invalid enum value (e.g., `level BOGUS`) → error, refuse to start
   - Invalid OS env var → error, refuse to start (BREAKING CHANGE)
   - Out-of-range value (e.g., `port 99999`) → error, refuse to start

   **BREAKING CHANGE:** Current `loadFromEnv()` silently ignores parse errors.
   This change requires modifying `loadFromEnv()` to return error.
   Users with typos in env vars will now get startup failures.

   **Migration aid:** `ze bgp config check --env` validates env vars before upgrade.

4. **Case sensitivity:** Section and option names are case-insensitive.
   Values are case-insensitive for booleans and enums, case-sensitive for strings.

5. **Boolean values:** `true`, `false`, `1`, `0`, `yes`, `no`, `on`, `off`, `enable`, `disable` (case-insensitive)

6. **Enum validation:** Known enums are validated:
   - `log.level`: DEBUG, INFO, NOTICE, WARNING, ERR, CRITICAL
   - `api.encoder`: json, text

7. **Range validation:**
   - `tcp.port`: 1-65535
   - `tcp.attempts`: 0-1000
   - `bgp.openwait`: 1-3600
   - `reactor.speed`: 0.1-10.0

8. **String values with spaces:** Use quotes for paths with spaces:
   ```
   log {
       destination "/var/log/my app/ze.bgp.log";
   }
   ```

9. **Comments:** Standard `#` comments allowed inside environment block.

10. **Empty block/section:** `environment { }` and `environment { log { } }` are valid (no-op).

11. **Priority (highest to lowest):**
    ```
    1. OS env: ze.bgp.section.option=value
    2. OS env: zebgp_section_option=value
    3. Config: environment { section { option value; } }
    4. Defaults
    ```

### Table-Driven Option Setters

Use table-driven approach to reduce boilerplate:

```go
// internal/config/environment.go

// envOption defines how to set an environment option.
type envOption struct {
    setter   func(env *Environment, value string) error
    validate func(value string) error // optional
}

// envOptions maps section.option to its setter and validator.
var envOptions = map[string]map[string]envOption{
    "log": {
        "level": {
            setter:   func(e *Environment, v string) error { e.Log.Level = strings.ToUpper(v); return nil },
            validate: validateLogLevel,
        },
        "enable": {
            setter: func(e *Environment, v string) error {
                b, err := parseBoolStrict(v)
                if err != nil { return err }
                e.Log.Enable = b
                return nil
            },
        },
        "destination": {
            setter: func(e *Environment, v string) error { e.Log.Destination = v; return nil },
        },
        // ... all other log options
    },
    "tcp": {
        "port": {
            setter: func(e *Environment, v string) error {
                n, err := strconv.Atoi(v)
                if err != nil { return fmt.Errorf("invalid port: %w", err) }
                e.TCP.Port = n
                return nil
            },
            validate: validatePort,
        },
        // ... all other tcp options
    },
    // ... all other sections
}

// validateLogLevel checks log level is valid.
func validateLogLevel(value string) error {
    valid := map[string]bool{
        "DEBUG": true, "INFO": true, "NOTICE": true,
        "WARNING": true, "ERR": true, "CRITICAL": true,
    }
    if !valid[strings.ToUpper(value)] {
        return fmt.Errorf("invalid log level %q: must be DEBUG, INFO, NOTICE, WARNING, ERR, or CRITICAL", value)
    }
    return nil
}

// validatePort checks port is in valid range.
func validatePort(value string) error {
    n, err := strconv.Atoi(value)
    if err != nil {
        return fmt.Errorf("invalid port %q: %w", value, err)
    }
    if n < 1 || n > 65535 {
        return fmt.Errorf("port %d out of range: must be 1-65535", n)
    }
    return nil
}

// validateEncoder checks encoder is valid.
func validateEncoder(value string) error {
    valid := map[string]bool{"json": true, "text": true}
    if !valid[strings.ToLower(value)] {
        return fmt.Errorf("invalid encoder %q: must be json or text", value)
    }
    return nil
}
```

### Strict Parsing Functions

Replace silent-ignore functions with error-returning versions:

```go
// parseBoolStrict parses a boolean value strictly.
// Returns error for invalid values instead of defaulting to false.
func parseBoolStrict(value string) (bool, error) {
    v := strings.ToLower(value)
    switch v {
    case "1", "true", "yes", "on", "enable":
        return true, nil
    case "0", "false", "no", "off", "disable":
        return false, nil
    default:
        return false, fmt.Errorf("invalid boolean %q: must be true/false/yes/no/on/off/enable/disable/1/0", value)
    }
}

// parseIntStrict parses an integer strictly.
func parseIntStrict(value string) (int, error) {
    n, err := strconv.Atoi(value)
    if err != nil {
        return 0, fmt.Errorf("invalid integer %q: %w", value, err)
    }
    return n, nil
}

// parseFloatStrict parses a float strictly.
func parseFloatStrict(value string) (float64, error) {
    f, err := strconv.ParseFloat(value, 64)
    if err != nil {
        return 0, fmt.Errorf("invalid float %q: %w", value, err)
    }
    return f, nil
}

// parseOctalStrict parses an octal integer strictly.
func parseOctalStrict(value string) (int, error) {
    v := strings.TrimPrefix(value, "0")
    n, err := strconv.ParseInt(v, 8, 32)
    if err != nil {
        return 0, fmt.Errorf("invalid octal %q: %w", value, err)
    }
    return int(n), nil
}
```

### API Changes

```go
// internal/config/environment.go

// SetConfigValue applies a single config value from the environment block.
// Returns error for unknown section/option, type parse failure, or validation failure.
func (e *Environment) SetConfigValue(section, option, value string) error {
    section = strings.ToLower(section)
    option = strings.ToLower(option)

    sectionOpts, ok := envOptions[section]
    if !ok {
        return fmt.Errorf("unknown environment section: %s", section)
    }

    opt, ok := sectionOpts[option]
    if !ok {
        return fmt.Errorf("unknown %s option: %s", section, option)
    }

    // Validate if validator exists
    if opt.validate != nil {
        if err := opt.validate(value); err != nil {
            return err
        }
    }

    // Set the value
    return opt.setter(e, value)
}

// loadFromEnvStrict loads values from environment variables with strict validation.
// Returns error on any parse failure instead of silently using defaults.
func (e *Environment) loadFromEnvStrict() error {
    for section, opts := range envOptions {
        for option := range opts {
            value := getEnv(section, option)
            if value == "" {
                continue
            }
            if err := e.SetConfigValue(section, option, value); err != nil {
                return fmt.Errorf("env var zebgp.%s.%s: %w", section, option, err)
            }
        }
    }
    return nil
}

// LoadEnvironmentWithConfig loads env: defaults → config block → OS env.
// The configValues map is section -> option -> value from parsed config.
func LoadEnvironmentWithConfig(configValues map[string]map[string]string) (*Environment, error) {
    env := &Environment{}
    env.loadDefaults()

    // Apply config file values
    for section, options := range configValues {
        for option, value := range options {
            if err := env.SetConfigValue(section, option, value); err != nil {
                return nil, fmt.Errorf("config environment.%s.%s: %w", section, option, err)
            }
        }
    }

    // OS env vars override config (with strict validation)
    if err := env.loadFromEnvStrict(); err != nil {
        return nil, err
    }

    return env, nil
}

// LoadEnvironment loads configuration from environment variables.
// Now returns error for invalid env vars (BREAKING CHANGE).
func LoadEnvironment() (*Environment, error) {
    return LoadEnvironmentWithConfig(nil)
}
```

### Schema Addition

```go
// internal/config/schema.go - add to sectionParsers

"environment": parseEnvironment,

// parseEnvironment handles environment { section { key value; } } blocks
func parseEnvironment(p *Parser, tokens []string) error {
    if p.config.Environment != nil {
        return fmt.Errorf("multiple environment blocks not allowed")
    }
    p.config.Environment = make(map[string]map[string]string)

    // Parse nested section blocks
    // Returns map[section]map[option]value
    return p.parseEnvironmentBlock()
}

func (p *Parser) parseEnvironmentBlock() error {
    for {
        tok := p.nextToken()
        if tok == "}" {
            return nil
        }

        section := strings.ToLower(tok)
        if !isValidEnvSection(section) {
            return fmt.Errorf("unknown environment section: %s", section)
        }

        if err := p.expect("{"); err != nil {
            return err
        }

        if p.config.Environment[section] == nil {
            p.config.Environment[section] = make(map[string]string)
        }

        if err := p.parseEnvironmentSection(section); err != nil {
            return err
        }
    }
}

func (p *Parser) parseEnvironmentSection(section string) error {
    for {
        tok := p.nextToken()
        if tok == "}" {
            return nil
        }

        option := strings.ToLower(tok)
        value := p.nextToken()

        // Validate option exists
        if _, ok := envOptions[section][option]; !ok {
            return fmt.Errorf("unknown %s option: %s", section, option)
        }

        // Validate value (fail-fast)
        if opt := envOptions[section][option]; opt.validate != nil {
            if err := opt.validate(value); err != nil {
                return fmt.Errorf("environment.%s.%s: %w", section, option, err)
            }
        }

        p.config.Environment[section][option] = value

        if err := p.expect(";"); err != nil {
            return err
        }
    }
}

func isValidEnvSection(section string) bool {
    _, ok := envOptions[section]
    return ok
}
```

### Migration Command

Add `--env` flag to `ze bgp config check`:

```go
// cmd/ze/bgp/config_check.go

func runConfigCheck(cmd *cobra.Command, args []string) error {
    checkEnv, _ := cmd.Flags().GetBool("env")

    if checkEnv {
        // Validate environment variables only
        env := &config.Environment{}
        env.loadDefaults()
        if err := env.loadFromEnvStrict(); err != nil {
            return fmt.Errorf("environment validation failed: %w", err)
        }
        fmt.Println("✅ Environment variables valid")
        return nil
    }

    // ... existing config check logic
}

func init() {
    configCheckCmd.Flags().Bool("env", false, "validate environment variables only")
}
```

### Integration Point

In `internal/config/loader.go`, when creating reactor config:

```go
func LoadReactorFile(path string) (*ReactorConfig, error) {
    // Parse config file including environment block
    cfg, err := parseConfigFile(path)
    if err != nil {
        return nil, err
    }

    // Load environment with config values
    env, err := LoadEnvironmentWithConfig(cfg.Environment)
    if err != nil {
        return nil, err
    }

    // Create reactor with environment
    return CreateReactorWithEnv(cfg, env)
}
```

## Implementation Steps

### Phase 1: Tests (TDD)

1. Add `TestParseBoolStrict` - strict boolean parsing with error
2. Add `TestParseIntStrict` - strict int parsing with error
3. Add `TestSetConfigValue` - test setting individual values via table
4. Add `TestSetConfigValueUnknownSection` - error on unknown section
5. Add `TestSetConfigValueUnknownOption` - error on unknown option
6. Add `TestSetConfigValueInvalidType` - error on type mismatch
7. Add `TestSetConfigValueInvalidEnum` - error on invalid log level/encoder
8. Add `TestSetConfigValueInvalidRange` - error on out-of-range port
9. Add `TestLoadEnvironmentWithConfig` - test combined loading
10. Add `TestConfigPriority` - verify OS env > config > defaults
11. Add `TestUnderscoreEnvPriority` - verify underscore env also beats config
12. Add `TestLoadFromEnvStrict` - verify env var errors propagate
13. Add `TestCaseInsensitive` - section/option names case-insensitive
14. Add `TestEmptySection` - empty section within environment block
15. Add `TestParseEnvironmentBlock` - config parsing test
16. Add `TestMultipleEnvironmentBlocks` - error on multiple blocks

**Run tests → MUST FAIL**

### Phase 2: Implementation

1. Add strict parsing functions: `parseBoolStrict()`, `parseIntStrict()`, etc.
2. Add validation functions: `validateLogLevel()`, `validatePort()`, `validateEncoder()`
3. Add `envOptions` table mapping section.option to setter/validator
4. Add `SetConfigValue(section, option, value)` using table lookup
5. Add `loadFromEnvStrict()` that returns error
6. Add `LoadEnvironmentWithConfig()` function
7. Update `LoadEnvironment()` to return `(*Environment, error)`
8. Update ALL call sites of `LoadEnvironment()` for new signature
9. Add `parseEnvironment()` to config parser
10. Add `environment` to schema/section parsers
11. Add `--env` flag to `ze bgp config check`

**Run tests → MUST PASS**

### Phase 3: Verification

```bash
make test && make lint
```

### Phase 4: Documentation

1. **Create** `docs/architecture/config/ENVIRONMENT_BLOCK.md`:
   - TL;DR with key concepts
   - Full syntax reference (all sections, all options)
   - Enum values and ranges
   - Priority order with examples
   - Validation/fail-fast behavior
   - Complete examples

2. **Update** `docs/architecture/config/SYNTAX.md`:
   - Add `environment` to section types list
   - Link to ENVIRONMENT_BLOCK.md

3. **Update** `docs/architecture/config/ENVIRONMENT.md`:
   - Add config block as value source
   - Update priority order docs
   - Document breaking change

4. **Update** `.claude/INDEX.md`:
   - Add ENVIRONMENT_BLOCK.md to Quick Navigation
   - Add to Documentation Tree

5. **Update** `docs/plan/CLAUDE_CONTINUATION.md`

## Test Cases

### Strict Parsing Tests

```go
func TestParseBoolStrict(t *testing.T) {
    tests := []struct {
        input   string
        want    bool
        wantErr bool
    }{
        {"true", true, false},
        {"false", false, false},
        {"TRUE", true, false},  // case insensitive
        {"False", false, false},
        {"1", true, false},
        {"0", false, false},
        {"yes", true, false},
        {"no", false, false},
        {"on", true, false},
        {"off", false, false},
        {"enable", true, false},
        {"disable", false, false},
        {"maybe", false, true},     // error
        {"", false, true},          // error
        {"2", false, true},         // error
        {"enabled", false, true},   // error - not exact match
    }

    for _, tt := range tests {
        got, err := parseBoolStrict(tt.input)
        if (err != nil) != tt.wantErr {
            t.Errorf("parseBoolStrict(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
            continue
        }
        if !tt.wantErr && got != tt.want {
            t.Errorf("parseBoolStrict(%q) = %v, want %v", tt.input, got, tt.want)
        }
    }
}

func TestValidateLogLevel(t *testing.T) {
    valid := []string{"DEBUG", "debug", "INFO", "info", "NOTICE", "WARNING", "ERR", "CRITICAL"}
    for _, v := range valid {
        if err := validateLogLevel(v); err != nil {
            t.Errorf("validateLogLevel(%q) unexpected error: %v", v, err)
        }
    }

    invalid := []string{"TRACE", "ERROR", "WARN", "FATAL", ""}
    for _, v := range invalid {
        if err := validateLogLevel(v); err == nil {
            t.Errorf("validateLogLevel(%q) expected error", v)
        }
    }
}

func TestValidatePort(t *testing.T) {
    valid := []string{"1", "179", "1179", "65535"}
    for _, v := range valid {
        if err := validatePort(v); err != nil {
            t.Errorf("validatePort(%q) unexpected error: %v", v, err)
        }
    }

    invalid := []string{"0", "65536", "-1", "abc", ""}
    for _, v := range invalid {
        if err := validatePort(v); err == nil {
            t.Errorf("validatePort(%q) expected error", v)
        }
    }
}
```

### SetConfigValue Tests

```go
func TestSetConfigValue(t *testing.T) {
    env := &Environment{}
    env.loadDefaults()

    tests := []struct {
        section string
        option  string
        value   string
        check   func() bool
    }{
        {"log", "level", "DEBUG", func() bool { return env.Log.Level == "DEBUG" }},
        {"LOG", "LEVEL", "INFO", func() bool { return env.Log.Level == "INFO" }}, // case insensitive
        {"tcp", "port", "1179", func() bool { return env.TCP.Port == 1179 }},
        {"bgp", "passive", "true", func() bool { return env.BGP.Passive }},
        {"bgp", "passive", "false", func() bool { return !env.BGP.Passive }},
        {"api", "encoder", "text", func() bool { return env.API.Encoder == "text" }},
        {"reactor", "speed", "2.5", func() bool { return env.Reactor.Speed == 2.5 }},
    }

    for _, tt := range tests {
        if err := env.SetConfigValue(tt.section, tt.option, tt.value); err != nil {
            t.Errorf("SetConfigValue(%q, %q, %q) error: %v", tt.section, tt.option, tt.value, err)
            continue
        }
        if !tt.check() {
            t.Errorf("SetConfigValue(%q, %q, %q) did not set correctly", tt.section, tt.option, tt.value)
        }
    }
}

func TestSetConfigValueErrors(t *testing.T) {
    env := &Environment{}
    env.loadDefaults()

    tests := []struct {
        section string
        option  string
        value   string
        errMsg  string
    }{
        {"invalid", "foo", "bar", "unknown environment section"},
        {"log", "invalid_option", "bar", "unknown log option"},
        {"tcp", "port", "abc", "invalid port"},
        {"tcp", "port", "99999", "out of range"},
        {"tcp", "port", "0", "out of range"},
        {"log", "level", "BOGUS", "invalid log level"},
        {"api", "encoder", "xml", "invalid encoder"},
        {"bgp", "passive", "maybe", "invalid boolean"},
    }

    for _, tt := range tests {
        err := env.SetConfigValue(tt.section, tt.option, tt.value)
        if err == nil {
            t.Errorf("SetConfigValue(%q, %q, %q) expected error containing %q", tt.section, tt.option, tt.value, tt.errMsg)
            continue
        }
        if !strings.Contains(err.Error(), tt.errMsg) {
            t.Errorf("SetConfigValue(%q, %q, %q) error = %q, want containing %q", tt.section, tt.option, tt.value, err.Error(), tt.errMsg)
        }
    }
}
```

### Priority Tests

```go
func TestConfigPriority(t *testing.T) {
    // Config sets DEBUG, OS env sets WARNING
    // OS env should win
    t.Setenv("ze.bgp.log.level", "WARNING")

    cfg := map[string]map[string]string{
        "log": {"level": "DEBUG"},
    }
    env, err := LoadEnvironmentWithConfig(cfg)
    if err != nil {
        t.Fatal(err)
    }

    if env.Log.Level != "WARNING" {
        t.Errorf("Log.Level = %q, want WARNING (OS env priority)", env.Log.Level)
    }
}

func TestUnderscoreEnvPriority(t *testing.T) {
    // Underscore env var also beats config
    t.Setenv("ze_bgp_log_level", "ERR")

    cfg := map[string]map[string]string{
        "log": {"level": "DEBUG"},
    }
    env, err := LoadEnvironmentWithConfig(cfg)
    if err != nil {
        t.Fatal(err)
    }

    if env.Log.Level != "ERR" {
        t.Errorf("Log.Level = %q, want ERR (underscore env priority)", env.Log.Level)
    }
}

func TestDotEnvBeatUnderscoreEnv(t *testing.T) {
    // Dot notation beats underscore notation
    t.Setenv("ze.bgp.log.level", "DEBUG")
    t.Setenv("ze_bgp_log_level", "ERR")

    env, err := LoadEnvironmentWithConfig(nil)
    if err != nil {
        t.Fatal(err)
    }

    if env.Log.Level != "DEBUG" {
        t.Errorf("Log.Level = %q, want DEBUG (dot beats underscore)", env.Log.Level)
    }
}
```

### Environment Variable Strict Validation Tests

```go
func TestLoadFromEnvStrictError(t *testing.T) {
    // Invalid env var should cause error
    t.Setenv("ze.bgp.tcp.port", "not_a_number")

    _, err := LoadEnvironment()
    if err == nil {
        t.Error("expected error for invalid env var")
    }
    if !strings.Contains(err.Error(), "ze.bgp.tcp.port") {
        t.Errorf("error should mention the env var, got: %v", err)
    }
}

func TestLoadFromEnvStrictInvalidEnum(t *testing.T) {
    t.Setenv("ze.bgp.log.level", "BOGUS")

    _, err := LoadEnvironment()
    if err == nil {
        t.Error("expected error for invalid log level")
    }
}
```

### Config Parsing Tests

```go
func TestParseEnvironmentBlock(t *testing.T) {
    config := `
environment {
    log {
        level DEBUG;
        short false;
    }
    tcp {
        port 1179;
    }
}

peer 192.0.2.1 {
    router-id 192.0.2.100;
    local-as 65000;
    peer-as 65001;
}
`
    cfg, err := ParseConfig(strings.NewReader(config))
    if err != nil {
        t.Fatal(err)
    }

    // Verify environment was parsed
    if cfg.Environment["log"]["level"] != "DEBUG" {
        t.Error("environment log.level not parsed")
    }
    if cfg.Environment["tcp"]["port"] != "1179" {
        t.Error("environment tcp.port not parsed")
    }
}

func TestParseEnvironmentBlockCaseInsensitive(t *testing.T) {
    config := `
environment {
    LOG {
        LEVEL debug;
    }
}
`
    cfg, err := ParseConfig(strings.NewReader(config))
    if err != nil {
        t.Fatal(err)
    }

    // Should be stored lowercase
    if cfg.Environment["log"]["level"] != "debug" {
        t.Error("environment log.level not parsed case-insensitively")
    }
}

func TestParseEmptySection(t *testing.T) {
    config := `
environment {
    log { }
    tcp {
        port 1179;
    }
}
`
    cfg, err := ParseConfig(strings.NewReader(config))
    if err != nil {
        t.Fatalf("empty section should be valid: %v", err)
    }

    if len(cfg.Environment["log"]) != 0 {
        t.Error("empty log section should have no entries")
    }
    if cfg.Environment["tcp"]["port"] != "1179" {
        t.Error("tcp.port not parsed")
    }
}

func TestMultipleEnvironmentBlocks(t *testing.T) {
    config := `
environment { log { level DEBUG; } }
environment { tcp { port 1179; } }
`
    _, err := ParseConfig(strings.NewReader(config))
    if err == nil {
        t.Error("expected error for multiple environment blocks")
    }
    if !strings.Contains(err.Error(), "multiple") {
        t.Errorf("error should mention 'multiple', got: %v", err)
    }
}

func TestEmptyEnvironmentBlock(t *testing.T) {
    config := `
environment { }

peer 192.0.2.1 {
    router-id 192.0.2.100;
    local-as 65000;
    peer-as 65001;
}
`
    cfg, err := ParseConfig(strings.NewReader(config))
    if err != nil {
        t.Fatalf("empty environment block should be valid: %v", err)
    }

    if len(cfg.Environment) != 0 {
        t.Errorf("expected empty environment map, got %d entries", len(cfg.Environment))
    }
}

func TestParseUnknownSection(t *testing.T) {
    config := `
environment {
    invalid {
        foo bar;
    }
}
`
    _, err := ParseConfig(strings.NewReader(config))
    if err == nil {
        t.Error("expected error for unknown section")
    }
}

func TestParseUnknownOption(t *testing.T) {
    config := `
environment {
    log {
        invalid_option value;
    }
}
`
    _, err := ParseConfig(strings.NewReader(config))
    if err == nil {
        t.Error("expected error for unknown option")
    }
}

func TestParseInvalidValue(t *testing.T) {
    config := `
environment {
    tcp {
        port abc;
    }
}
`
    _, err := ParseConfig(strings.NewReader(config))
    if err == nil {
        t.Error("expected error for invalid port value")
    }
}
```

### All Sections Test

```go
func TestAllSections(t *testing.T) {
    cfg := map[string]map[string]string{
        "daemon":  {"user": "ze-bgp", "daemonize": "true"},
        "log":     {"level": "DEBUG", "short": "false"},
        "tcp":     {"port": "1179", "attempts": "5"},
        "bgp":     {"passive": "true", "openwait": "120"},
        "cache":   {"attributes": "false"},
        "api":     {"encoder": "text", "respawn": "false"},
        "reactor": {"speed": "2.0"},
        "debug":   {"timing": "true"},
    }
    env, err := LoadEnvironmentWithConfig(cfg)
    if err != nil {
        t.Fatal(err)
    }

    // Verify all sections applied
    if env.Daemon.User != "ze-bgp" { t.Error("Daemon.User") }
    if !env.Daemon.Daemonize { t.Error("Daemon.Daemonize") }
    if env.Log.Level != "DEBUG" { t.Error("Log.Level") }
    if env.Log.Short { t.Error("Log.Short should be false") }
    if env.TCP.Port != 1179 { t.Error("TCP.Port") }
    if env.TCP.Attempts != 5 { t.Error("TCP.Attempts") }
    if !env.BGP.Passive { t.Error("BGP.Passive") }
    if env.BGP.OpenWait != 120 { t.Error("BGP.OpenWait") }
    if env.Cache.Attributes { t.Error("Cache.Attributes should be false") }
    if env.API.Encoder != "text" { t.Error("API.Encoder") }
    if env.API.Respawn { t.Error("API.Respawn should be false") }
    if env.Reactor.Speed != 2.0 { t.Error("Reactor.Speed") }
    if !env.Debug.Timing { t.Error("Debug.Timing") }
}
```

## Checklist

### Implementation
- [ ] Strict parsing functions written
- [ ] Validation functions written
- [ ] envOptions table complete
- [ ] Tests written first
- [ ] Tests fail before impl
- [ ] Tests pass after impl
- [ ] `LoadEnvironment()` signature updated
- [ ] All call sites updated
- [ ] `--env` flag added to config check
- [ ] `make test` passes
- [ ] `make lint` passes
- [ ] Goal achieved (environment from config works)

### Documentation
- [ ] `docs/architecture/config/ENVIRONMENT_BLOCK.md` created
- [ ] `docs/architecture/config/SYNTAX.md` updated
- [ ] `docs/architecture/config/ENVIRONMENT.md` updated
- [ ] `.claude/INDEX.md` updated
- [ ] `docs/plan/CLAUDE_CONTINUATION.md` updated

### Completion
- [ ] Spec moved to `docs/plan/done/`
- [ ] `docs/plan/README.md` updated

---

**Created:** 2025-12-30
**Updated:** 2025-12-31 (critical review fixes)
