"""ze_plugin: a small library for writing an external Ze plugin in Python.

An external plugin is a program Ze starts from a `bus { endpoint <name> {
external { run ...; } } }` block. This library lets that program bring its own YANG
modules, be told its config at start and on each commit, and serve the
commands its modules define:

    from ze_plugin import Plugin, ConfigError, CommandError

    plugin = Plugin("acme-monitor")
    plugin.schema(CONF_YANG)    # acme-monitor-conf: the config section
    plugin.schema(CMD_YANG)     # acme-monitor-cmd: the commands and their arguments
    plugin.schema(API_YANG)     # acme-monitor-api: the rpcs MCP, REST and gRPC read

    @plugin.config("acme-monitor")
    class Monitor:
        def verify(self, new): ...            # raise ConfigError to refuse
        def apply(self, new, changed): ...    # at start and on each commit
        def rollback(self, old): ...

    @plugin.command("show acme-monitor target")
    def target(args):                         # args: the values Ze checked
        raise CommandError("no such target", fix="show acme-monitor status")

    plugin.run()

The wire contract is docs/plugin-development/protocol.md. Where that page is
silent, the Go code is the authority, and each step below names the page
section or the Go function it mirrors.

What it does: declare mode (ZE_PLUGIN_MODE=declare), the TLS connect-back and
the `#0 auth` exchange, the five startup stages, config verify, apply, rollback
and committed, execute-command, bye.

What it does not do yet:
- It subscribes to no event and calls no engine RPC after startup (no route
  update, no dispatch-command), so it never reads a record answer of its own.
- It declares no family, capability, filter, doctor check or metric.
- It serves one request at a time, on one thread. A handler that blocks delays
  every later request, so a handler must answer from what it holds.
- It streams no rows: a command answer is one document, so it must fit one
  wire message (16 MiB).

Python 3.9 or later, standard library only. Run a plugin as
`python3 my_plugin.py`: the file carries no shebang.
"""

import json
import os
import re
import socket
import ssl
import sys
import traceback

__all__ = ["Plugin", "Config", "ConfigError", "CommandError"]

# pkg/plugin/rpc/types.go ProtocolVersion: the one version this library speaks.
PROTOCOL_VERSION = 1
# pkg/plugin/rpc/framing.go MaxMessageSize: no line is wider than this.
MAX_MESSAGE_SIZE = 16 * 1024 * 1024

_ENGINE = "ze-plugin-engine:"
_CALLBACK = "ze-plugin-callback:"

# A copy of command.Verbs (internal/component/command/verbs.go). The engine is
# the authority: the conformance row `command-names` judges the declared names
# with the engine's own list, so a verb missing here shows there.
_VERBS = frozenset(
    "show monitor resolve set delete clear request commit update cache create send debug".split()
)
# internal/component/command/grammar/checker.go mutationTokens (rule R7).
_MUTATION_TOKENS = frozenset(("add", "remove"))
_TOKEN = re.compile(r"[a-z0-9]+(?:-[a-z0-9]+)*\Z")

_ID = re.compile(rb"#(\d{1,20}) ")
_DIGITS = re.compile(rb"\d*")

# pkg/plugin/rpc/message.go answerLineShapes: the field order of each answer
# kind. w is a three-letter word, n a counted number, t a counted text. The key
# is the kind and the space that closes it (answerKindAt).
_ANSWER_SHAPES = {
    b"top ": "wtt",
    b"row ": "t",
    b"bad ": "t",
    b"end ": "nnt",
    b"wrn ": "nnt",
    b"nay ": "tt",
}
_OTHER, _PARTIAL, _STATED = range(3)

_REQUIRED = object()
_SENT = object()


class _Refusal(Exception):
    """A refusal the plugin sends to Ze: what is wrong, and what clears it.

    `message` says what is wrong and why. `fix` is the command or config
    statement that clears it. The text sent to Ze carries the fix on its last
    line, under the `fix: ` label the engine's own errors use
    (internal/core/opererr/opererr.go Error.Error).
    """

    def __init__(self, message, fix=None):
        super().__init__(message)
        self.message = message
        self.fix = fix

    def __str__(self):
        if self.fix:
            return self.message + "\nfix: " + self.fix
        return self.message


class ConfigError(_Refusal):
    """A config the plugin refuses, or a config value that does not convert."""


class CommandError(_Refusal):
    """A command the plugin cannot answer, such as one that names no known item."""


class Config:
    """One config section, as Ze delivered it. Read-only.

    Ze delivers every leaf value as a string. `get` returns it as it came, and
    `int` and `bool` convert on request and raise ConfigError on a value that
    does not convert. A nested section comes back as a Config, and a list keeps
    its order, with each nested section in it wrapped the same way.

    An EMPTY Config means the section is absent: the operator removed it, or
    never wrote it. `if not new:` tests for that.
    """

    def __init__(self, body=None, path=""):
        self._body = dict(body) if isinstance(body, dict) else {}
        self._path = path

    def _where(self, key):
        return self._path + "/" + key if self._path else key

    def _wrap(self, key, value):
        if isinstance(value, dict):
            return Config(value, self._where(key))
        if isinstance(value, list):
            return [Config(item, self._where(key)) if isinstance(item, dict) else item for item in value]
        return value

    def get(self, key, default=None):
        """The value of `key` as delivered, or `default` when it is absent."""
        if key not in self._body:
            return default
        return self._wrap(key, self._body[key])

    def int(self, key, default=_REQUIRED):
        """The value of `key` as an integer.

        An absent key answers `default`, and raises ConfigError when no default
        was given, so a missing value never reads as zero.
        """
        value = self._body.get(key)
        if value is None:
            return self._absent(key, default)
        if isinstance(value, int) and not isinstance(value, bool):
            return value
        if isinstance(value, str) and re.fullmatch(r"[+-]?[0-9]+", value):
            return int(value)
        raise ConfigError("%s is %r, which is not a whole number" % (self._where(key), value))

    def bool(self, key, default=_REQUIRED):
        """The value of `key` as a boolean: `true` or `false`."""
        value = self._body.get(key)
        if value is None:
            return self._absent(key, default)
        if value is True or value == "true":
            return True
        if value is False or value == "false":
            return False
        raise ConfigError("%s is %r, which is neither true nor false" % (self._where(key), value))

    def _absent(self, key, default):
        if default is _REQUIRED:
            raise ConfigError("%s has no value, and the plugin needs one" % self._where(key))
        return default

    def to_dict(self):
        """A copy of the section body, as plain Python values."""
        return json.loads(json.dumps(self._body))

    def keys(self):
        return self._body.keys()

    def items(self):
        return [(key, self._wrap(key, value)) for key, value in self._body.items()]

    def __getitem__(self, key):
        return self._wrap(key, self._body[key])

    def __contains__(self, key):
        return key in self._body

    def __iter__(self):
        return iter(self._body)

    def __len__(self):
        return len(self._body)

    def __eq__(self, other):
        return isinstance(other, Config) and self._body == other._body

    def __repr__(self):
        return "Config(%r)" % (self._body,)


class _Closed(Exception):
    """The engine closed the connection."""


class _Fatal(Exception):
    """The plugin cannot start or cannot go on. The text goes to stderr."""


def _compact(value):
    # Compact JSON, ASCII only: the declaration line allows no white space
    # between tokens (protocol.md, "Write the declaration line alone").
    return json.dumps(value, separators=(",", ":")).encode("ascii")


def _request_line(request_id, method, params):
    # pkg/plugin/rpc/message.go AppendRequest: `#<id> <method> [<json>]`.
    line = b"#%d %s" % (request_id, method.encode("ascii"))
    if params is not None:
        line += b" " + _compact(params)
    return line + b"\n"


def _answer_width(data):
    """How many bytes the answer line opening `data` occupies.

    Mirrors pkg/plugin/rpc/message.go answerLineWidth. An answer line states
    its own width, field by field, so a value in it may hold a raw newline.
    _OTHER means the bytes open no answer line and the newline frames them.
    _PARTIAL means an answer line whose width has not arrived yet.
    """
    at = 0
    if data[:1] == b"#":
        found = _ID.match(data)
        if not found:
            return _OTHER, 0
        at = found.end()
    shapes = _ANSWER_SHAPES.get(bytes(data[at:at + 4]))
    if shapes is None:
        return _OTHER, 0
    at += 4
    for index, shape in enumerate(shapes):
        if index:
            if at >= len(data):
                return _PARTIAL, 0
            if data[at] != 0x20:
                return _OTHER, 0
            at += 1
        left = len(data) - at
        if shape == "w":
            if left < 3:
                return _PARTIAL, 0
            at += 3
            continue
        digits = _DIGITS.match(data, at).end() - at
        if shape == "n":
            if digits >= left:
                return _PARTIAL, 0
            if not 1 <= digits <= 20:
                return _OTHER, 0
            at += digits
            continue
        # A counted text: `<n>:<n bytes>` (countedTextAt, countedTextState).
        if not 1 <= digits <= 20 or digits >= left or data[at + digits] != 0x3A:
            return (_PARTIAL if left <= 20 else _OTHER), 0
        size = int(bytes(data[at:at + digits]))
        if size > MAX_MESSAGE_SIZE:
            return _STATED, size
        at += digits + 1 + size
    return _STATED, at


def _check_command_name(name):
    # internal/component/plugin/server/command_registry.go validateCommandName.
    if not isinstance(name, str) or not name:
        raise ValueError("a command needs a name, such as \"show acme-monitor status\"")
    if name != name.strip():
        raise ValueError("command name %r has leading or trailing white space" % name)
    tokens = name.split(" ")
    for token in tokens:
        if not token:
            raise ValueError("command name %r holds two spaces in a row: write one space between words" % name)
        if not _TOKEN.match(token):
            raise ValueError(
                "command name %r holds the word %r: a word is lowercase ASCII letters, digits, "
                "and hyphens inside it" % (name, token)
            )
    if tokens[0] not in _VERBS:
        raise ValueError(
            "command name %r starts with %r, which is not a verb: start it with one of %s"
            % (name, tokens[0], ", ".join(sorted(_VERBS)))
        )
    for token in tokens:
        if token in _MUTATION_TOKENS:
            raise ValueError(
                "command name %r holds the word %r: config changes go through set and delete, "
                "so a command name cannot hold add or remove" % (name, token)
            )


def _check_help(name, text):
    # internal/component/plugin/server/startup.go validateHelpDecls: the summary
    # is one line, at most 256 bytes, with no control character.
    if not isinstance(text, str):
        raise ValueError("the help of command %r must be a string" % name)
    if len(text.encode("utf-8")) > 256:
        raise ValueError("the help of command %r is longer than 256 bytes: shorten it to one sentence" % name)
    if any(ord(char) < 0x20 or ord(char) == 0x7F for char in text):
        raise ValueError("the help of command %r holds a control character: write it on one line" % name)


def _section_body(root, data):
    # internal/component/config/plugin_verify.go ExtractConfigSubtree: the data
    # is the section wrapped in its full path, `{"<root>": {...}}`. A removed
    # root arrives as `{}` (reload_compensation.go reloadConfigSections).
    tree = json.loads(data) if data else {}
    for part in root.split("/"):
        tree = tree.get(part) if isinstance(tree, dict) else None
    # UNVERIFIED: a presence container that holds no leaf. If Ze delivers it as
    # anything but an object with members, it reads here as an absent section.
    return tree if isinstance(tree, dict) else {}


def _changed_paths(section):
    # internal/component/plugin/server/reload.go buildDiffSections: added,
    # removed and changed are each a JSON object keyed by config path.
    paths = set()
    for part in ("added", "removed", "changed"):
        text = section.get(part)
        if text:
            paths.update(json.loads(text).keys())
    return paths


class Plugin:
    """One external plugin: its schema, its config handlers and its commands."""

    def __init__(self, name):
        self.name = name
        # The name the operator configured, known once Stage 1 is answered.
        self.instance_name = ""
        self._schemas = []
        self._configs = {}
        self._commands = {}
        # Standard output carries the declaration line and nothing else, so it
        # is kept aside and print() goes to stderr from here on.
        self._declaration_out = getattr(sys.stdout, "buffer", sys.stdout)
        sys.stdout = sys.stderr
        self._sock = None
        self._buffer = bytearray()
        self._next_id = 0
        self._answers = {}
        self._configured = False
        self._registry_seen = False
        self._ready = False
        self._bye = False
        self._running = {}
        self._pending = {}
        self._undo = []
        self._apply_open = False
        self._callbacks = {
            _CALLBACK + "configure": self._on_configure,
            _CALLBACK + "share-registry": self._on_share_registry,
            _CALLBACK + "config-verify": self._on_config_verify,
            _CALLBACK + "config-apply": self._on_config_apply,
            _CALLBACK + "config-rollback": self._on_config_rollback,
            _CALLBACK + "config-committed": self._on_config_committed,
            _CALLBACK + "execute-command": self._on_execute_command,
            _CALLBACK + "bye": self._on_bye,
            # Answered OK with no handler, as the Go SDK does
            # (pkg/plugin/sdk/sdk_callbacks.go initCallbackDefaults).
            _CALLBACK + "deliver-event": self._accept,
            _CALLBACK + "deliver-batch": self._accept,
            _CALLBACK + "post-startup": self._accept,
        }

    # ---- what the author declares ----

    def log(self, text):
        """Write one line to stderr, which Ze relays to its own log."""
        sys.stderr.write(str(text).rstrip("\n") + "\n")
        sys.stderr.flush()

    def schema(self, yang_text):
        """Declare one YANG module this plugin brings. Call it once for each module.

        The module name and the namespace are read out of the text. A module
        whose name ends in `-conf` adds its top-level containers to the config,
        one whose name ends in `-cmd` adds the commands it declares, and the
        rpc statements of a module (an `-api` module) describe the parameters
        of the commands whose ze:rpc names them. Ze joins every module when
        `ze start` runs the plugin, and refuses the start when one does not
        compile or clashes with a module Ze holds
        (docs/plugin-development/commands.md).
        """
        module = re.search(r"^\s*module\s+([A-Za-z_][A-Za-z0-9_.-]*)\s*\{", yang_text, re.MULTILINE)
        if not module:
            raise ValueError(
                "the YANG text of plugin %s names no module: it must open with `module <name> {`" % self.name
            )
        namespace = re.search(r"\bnamespace\s+(?:\"([^\"]+)\"|'([^']+)'|([^\s;\"']+))\s*;", yang_text)
        if not namespace:
            raise ValueError(
                "YANG module %s states no namespace: add `namespace \"urn:...\";` inside the module" % module.group(1)
            )
        if any(declared["module"] == module.group(1) for declared in self._schemas):
            raise ValueError("plugin %s already declared YANG module %s" % (self.name, module.group(1)))
        self._schemas.append({
            "module": module.group(1),
            "namespace": next(group for group in namespace.groups() if group),
            "yang-text": yang_text,
        })

    def config(self, root):
        """Class decorator: the class handles the config section named `root`.

        The class is created once, with no argument, and may define:

        verify(self, new)
            Called for the config at start and for each candidate of a commit.
            Raise ConfigError(message, fix=...) to refuse it. It must change
            nothing: a refused commit also runs it.
        apply(self, new, changed)
            Required. Called at start when the config holds the section, and on
            each accepted commit that changes it. `new` is the whole section.
            An EMPTY `new` means the section was removed. `changed` is the set
            of changed config paths Ze sent with the commit, such as
            `acme-monitor/interval`. At start there is no diff, so `changed`
            holds the root name alone. An apply that raises must leave what
            runs as it was.
        rollback(self, old)
            Called when a commit this plugin applied is undone, with the
            section as it was before. Without it, `apply(old, {root})` runs.
        """
        if not isinstance(root, str) or not root:
            raise ValueError("@plugin.config needs the name of a config root, such as \"acme-monitor\"")
        if root in self._configs:
            raise ValueError("config root %s already has a handler in plugin %s" % (root, self.name))

        def register(cls):
            # pkg/plugin/sdk/sdk_callbacks.go refuseWithoutApply: the engine
            # side refuses every reload for a plugin with no apply handler.
            if not callable(getattr(cls, "apply", None)):
                raise TypeError(
                    "%s handles config root %s and defines no apply(self, new, changed): Ze refuses every "
                    "commit that changes %s for a plugin that cannot apply it. Add an apply method."
                    % (cls.__name__, root, root)
                )
            self._configs[root] = cls()
            return cls

        return register

    def command(self, name, help=""):
        """Function decorator: the function serves the command `name`.

        The decorator lists the command in the Stage 1 `commands`, which is
        what makes Ze ask this plugin for it, and serves it when Ze asks.

        The name starts with a verb (show, request, clear, ...) and holds
        lowercase words. When a -cmd module of this plugin defines the
        command, the name is the path of the container that holds its
        ze:command, and the module carries its help (ze:help) and its
        arguments (leaves). Ze checks each value against its leaf's type
        before it asks the plugin, so the function gets only values the type
        accepts.

        `help` is the one-line `short-help` of the Stage 1 entry, at most 256
        bytes. It is optional, and is the only help of a command that no
        module of this plugin defines.

        The function takes the list of argument values and returns a list of
        dict rows, one dict, a string, or None. Raise CommandError(message,
        fix=...) to answer an error with the command that clears it. Any
        other exception answers an error with its text.
        """
        _check_command_name(name)
        _check_help(name, help)
        if name in self._commands:
            raise ValueError("command %r is declared twice in plugin %s" % (name, self.name))

        def register(function):
            self._commands[name] = (help, function)
            return function

        return register

    def registration(self):
        """The Stage 1 declaration (pkg/plugin/rpc/types.go DeclareRegistrationInput)."""
        declaration = {"protocol-version": PROTOCOL_VERSION}
        if self._commands:
            commands = []
            for name, (summary, _) in self._commands.items():
                # The wire key is `short-help`: the engine refuses a `help` key
                # (CommandDecl.RetiredHelp).
                commands.append({"name": name, "short-help": summary} if summary else {"name": name})
            declaration["commands"] = commands
        if self._configs:
            declaration["wants-config"] = list(self._configs)
        if self._schemas:
            declaration["schemas"] = [dict(schema) for schema in self._schemas]
        return declaration

    def run(self):
        """Answer a declaration query, or connect to Ze and serve. Exits the process."""
        # protocol.md, "Query Mode: Answering Without Starting": one line on
        # stdout, id 1, exit 0, nothing started (rpc.WriteDeclaration).
        if os.environ.get("ZE_PLUGIN_MODE") == "declare":
            self._declaration_out.write(_request_line(1, _ENGINE + "declare-registration", self.registration()))
            self._declaration_out.flush()
            sys.exit(0)
        try:
            self._serve()
        except _Closed:
            # protocol.md, "Error Handling": a closed connection after startup
            # is how a daemon stop ends every plugin.
            if not self._ready:
                self.log("%s: ze closed the connection before the plugin was ready" % self.name)
                sys.exit(1)
        except _Fatal as failure:
            self.log("%s: %s" % (self.name, failure))
            sys.exit(1)
        sys.exit(0)

    # ---- startup: pkg/plugin/sdk/sdk.go Plugin.Run ----

    def _serve(self):
        self._connect()
        # Stage 1: declare-registration. The engine answers the version both
        # sides speak (pkg/plugin/sdk/sdk_version.go acceptProtocolVersion).
        answer = self._call(_ENGINE + "declare-registration", self.registration()) or {}
        if answer.get("protocol-version") != PROTOCOL_VERSION:
            raise _Fatal(
                "ze answered plugin protocol version %r and this library speaks version %d only"
                % (answer.get("protocol-version"), PROTOCOL_VERSION)
            )
        self.instance_name = answer.get("plugin-name", "")
        # Stage 2: the engine sends configure.
        while not self._configured:
            self._pump()
        # Stage 3: declare-capabilities. This library declares none.
        self._call(_ENGINE + "declare-capabilities", {"capabilities": []})
        # Stage 4: the engine sends share-registry.
        while not self._registry_seen:
            self._pump()
        # Stage 5: ready, with no event subscription.
        self._call(_ENGINE + "ready", {})
        self._ready = True
        # pkg/plugin/sdk/sdk_dispatch.go eventLoop: serve until bye or close.
        while not self._bye:
            self._pump()

    def _connect(self):
        # pkg/plugin/sdk/sdk.go dialAndAuth, with the variables
        # internal/component/plugin/process/process.go startExternal sets.
        host = os.environ.get("ZE_PLUGIN_HUB_HOST") or "127.0.0.1"
        port = os.environ.get("ZE_PLUGIN_HUB_PORT") or "12700"
        token = os.environ.pop("ZE_PLUGIN_HUB_TOKEN", "")
        root = os.environ.get("ZE_PLUGIN_CA_PEM", "")
        # The hub binds the token to the name the operator configured
        # (internal/component/plugin/ipc/tls.go authenticateWithName), and
        # ZE_PLUGIN_NAME carries that name.
        name = os.environ.get("ZE_PLUGIN_NAME") or self.name
        started_by_ze = "ze starts this program from a `plugin { external <name> { run ...; } }` block; " \
            "run it with ZE_PLUGIN_MODE=declare to print its declaration instead"
        if not token:
            raise _Fatal("ZE_PLUGIN_HUB_TOKEN is not set, so the plugin cannot authenticate to ze: " + started_by_ze)
        if not root.strip():
            raise _Fatal("ZE_PLUGIN_CA_PEM is not set, so the plugin cannot verify ze: " + started_by_ze)
        try:
            # internal/component/plugin/ipc/tls.go TLSConfigWithRoot: the given
            # root is the only trust anchor, and TLS 1.3 is the minimum.
            # UNVERIFIED: that Python accepts the certificate ze presents for
            # this host name (check_hostname stays on, as Go's dialer has it).
            context = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
            context.minimum_version = ssl.TLSVersion.TLSv1_3
            context.load_verify_locations(cadata=root)
            plain = socket.create_connection((host, int(port)), timeout=10)
            plain.setsockopt(socket.IPPROTO_TCP, socket.TCP_NODELAY, 1)
            self._sock = context.wrap_socket(plain, server_hostname=host)
            # internal/component/plugin/ipc/tls.go SendAuth: `#0 auth {...}`,
            # answered `#0 ok` or `#0 error {...}`.
            self._sock.sendall(_request_line(0, "auth", {"token": token, "name": name}))
            _, rest = self._cut_id(self._read_line())
        except _Closed:
            raise _Fatal("ze closed the connection during authentication: the token or the plugin name was refused")
        except (OSError, ValueError) as failure:
            raise _Fatal("cannot connect to ze at %s port %s: %s" % (host, port, failure))
        verb, _, payload = rest.partition(b" ")
        if verb != b"ok":
            raise _Fatal("ze refused the authentication: " + payload.decode("utf-8", "replace"))
        self._sock.settimeout(None)

    # ---- framing: pkg/plugin/rpc/framing.go and mux.go ----

    def _read_line(self):
        # framing.go ScanAnswerLines: an answer line is taken by the width it
        # states, and every other line ends at its newline.
        while True:
            state, width = _answer_width(self._buffer)
            if state == _STATED:
                if width > MAX_MESSAGE_SIZE:
                    raise _Fatal("ze sent an answer line of %d bytes, past the %d-byte maximum" % (width, MAX_MESSAGE_SIZE))
                if len(self._buffer) > width:
                    if self._buffer[width] != 0x0A:
                        raise _Fatal("ze sent an answer line of %d bytes that does not end with a newline" % width)
                    return self._take(width)
            elif state == _OTHER:
                end = self._buffer.find(b"\n")
                if end >= 0:
                    return self._take(end)
                if len(self._buffer) > MAX_MESSAGE_SIZE:
                    raise _Fatal("ze sent a line past the %d-byte maximum" % MAX_MESSAGE_SIZE)
            try:
                chunk = self._sock.recv(65536)
            except socket.timeout:
                raise _Fatal("ze did not answer within 10 seconds")
            except OSError:
                raise _Closed()
            if not chunk:
                raise _Closed()
            self._buffer += chunk

    def _take(self, width):
        line = bytes(self._buffer[:width])
        del self._buffer[:width + 1]
        return line

    def _cut_id(self, line):
        # pkg/plugin/rpc/message.go cutID: `#<id> ` opens every line.
        found = _ID.match(line)
        if not found:
            raise ValueError("line states no readable id: %r" % line[:80])
        return int(found.group(1)), line[found.end():]

    def _send(self, data):
        try:
            self._sock.sendall(data)
        except OSError:
            raise _Closed()

    def _call(self, method, params):
        """Send one request to the engine and wait for its answer.

        The engine may send its own requests before it answers, and they are
        served here, so no request waits behind this call.
        """
        self._next_id += 1
        request_id = self._next_id
        self._send(_request_line(request_id, method, params))
        while request_id not in self._answers:
            self._pump()
        verb, payload = self._answers.pop(request_id)
        if verb == b"error":
            raise _Fatal("ze refused %s: %s" % (method, payload.decode("utf-8", "replace")))
        return json.loads(payload) if payload else None

    def _pump(self):
        """Read one line and route it, as pkg/plugin/rpc/mux.go readLoop does."""
        line = self._read_line()
        try:
            line_id, rest = self._cut_id(line)
        except ValueError as failure:
            self.log("%s: dropped a line from ze: %s" % (self.name, failure))
            return
        # An answer line states a three-letter kind. This library starts no
        # call that answers with records, so nobody waits for one
        # (mux.go discardOrphanResponse).
        if bytes(rest[:4]) in _ANSWER_SHAPES:
            return
        verb, _, payload = rest.partition(b" ")
        # `ok` and `error` answer one of this plugin's own requests. The two
        # directions number their requests apart, so the verb decides.
        if verb in (b"ok", b"error"):
            self._answers[line_id] = (verb, payload)
            return
        self._dispatch(line_id, verb.decode("ascii", "replace"), payload)

    def _dispatch(self, request_id, method, payload):
        handler = self._callbacks.get(method)
        if handler is None:
            # protocol.md, "Error Handling": an unknown method is refused.
            self._send_error(request_id, "unknown method: " + method)
            return
        try:
            result = handler(request_id, json.loads(payload) if payload else {})
        except _Closed:
            raise
        except _Refusal as refusal:
            self._send_error(request_id, str(refusal))
            return
        except Exception as failure:  # A handler's own defect: answer it, keep the connection.
            self._log_exception(method)
            self._send_error(request_id, "%s: %s" % (type(failure).__name__, failure))
            return
        if result is _SENT:
            return
        # pkg/plugin/rpc/message.go AppendResult: `#<id> ok [<json>]`.
        self._send(b"#%d ok" % request_id + (b" " + _compact(result) if result is not None else b"") + b"\n")

    def _send_error(self, request_id, message):
        # pkg/plugin/rpc/conn.go SendError: `#<id> error {"code","message"}`.
        self._send(b"#%d error " % request_id + _compact({"code": "error", "message": message}) + b"\n")

    def _log_exception(self, what):
        self.log("%s: unexpected error while serving %s\n%s" % (self.name, what, traceback.format_exc()))

    # ---- callbacks ----

    def _accept(self, request_id, params):
        return None

    def _on_share_registry(self, request_id, params):
        self._registry_seen = True
        return None

    def _on_bye(self, request_id, params):
        # protocol.md, "Runtime Callbacks": bye is acknowledged, then the
        # plugin exits (sdk_dispatch.go eventLoop).
        self._bye = True
        return None

    def _verify(self, root, new):
        verify = getattr(self._configs[root], "verify", None)
        if callable(verify):
            verify(new)

    def _on_configure(self, request_id, params):
        # Stage 2 (protocol.md, "Stage 2: Config"), and its redelivery at
        # runtime. A ConfigError out of here is the `#<id> error` answer the
        # engine reads as a refusal. Startup goes on either way, as in
        # sdk_dispatch.go serveOne: the engine decides what a refusal stops.
        try:
            for section in params.get("sections") or []:
                root = section.get("root")
                if root not in self._configs:
                    continue
                body = _section_body(root, section.get("data"))
                new = Config(body, root)
                self._verify(root, new)
                self._configs[root].apply(new, {root})
                self._running[root] = body
        finally:
            self._configured = True
        return None

    def _on_config_verify(self, request_id, params):
        # pkg/plugin/sdk/config_tx.go ConfigTx.Verify: the accepted candidate
        # is held for the apply, because config-apply carries a diff and not
        # the section. A refused candidate leaves nothing held.
        self._pending = {}
        candidates = {}
        try:
            for section in params.get("sections") or []:
                root = section.get("root")
                if root not in self._configs:
                    continue
                body = _section_body(root, section.get("data"))
                self._verify(root, Config(body, root))
                candidates[root] = body
        except ConfigError as refusal:
            return {"status": "error", "error": str(refusal)}
        except Exception as failure:
            self._log_exception("config-verify")
            return {"status": "error", "error": "%s: %s" % (type(failure).__name__, failure)}
        self._pending = candidates
        return {"status": "ok"}

    def _on_config_apply(self, request_id, params):
        # ConfigTx.Apply: install the verified candidate and keep what it
        # replaced. The undo set of an earlier apply belongs to a commit that
        # already landed, so it is dropped here.
        pending, self._pending = self._pending, {}
        self._undo = []
        applied = []
        try:
            for section in params.get("sections") or []:
                root = section.get("root")
                if root not in self._configs:
                    continue
                # UNVERIFIED: that every root of a config-apply was carried by
                # the config-verify before it. If not, the apply is refused
                # here (ConfigTx: ErrConfigApplyUnverified).
                if root not in pending:
                    raise ConfigError("config apply for %s requires a verified candidate, and none is held" % root)
                self._configs[root].apply(Config(pending[root], root), _changed_paths(section))
                applied.append((root, self._running.get(root, {})))
                self._running[root] = pending[root]
        except Exception as failure:
            reason = str(failure)
            if not isinstance(failure, ConfigError):
                self._log_exception("config-apply")
                reason = "%s: %s" % (type(failure).__name__, failure)
            # sdk_callbacks.go OnConfigApply: the engine's rollback does not
            # reach a plugin whose own apply failed, so the roots this call
            # already applied are put back here.
            try:
                self._roll_back(applied)
            except Exception:
                self._log_exception("the undo of a failed config-apply")
            return {"status": "error", "error": reason}
        # pkg/plugin/sdk/config_tx_gate.go: an accepted apply stays open until
        # config-committed or config-rollback closes it.
        self._undo = applied
        self._apply_open = True
        return {"status": "ok"}

    def _roll_back(self, applied):
        # ConfigTx.Rollback: install back what the apply replaced.
        for root, old in reversed(applied):
            handler = self._configs[root]
            previous = Config(old, root)
            if callable(getattr(handler, "rollback", None)):
                handler.rollback(previous)
            else:
                handler.apply(previous, {root})
            self._running[root] = old

    def _on_config_rollback(self, request_id, params):
        # config_tx_gate.go rollbackOwed: the engine sends this to every
        # participant of a failed commit, and only a plugin with an open apply
        # has something to undo.
        self._pending = {}
        if self._apply_open:
            self._apply_open = False
            undo, self._undo = self._undo, []
            self._roll_back(undo)
        return None

    def _on_config_committed(self, request_id, params):
        # config_tx_gate.go committed: the open apply is permanent.
        self._apply_open = False
        self._undo = []
        return None

    def _on_execute_command(self, request_id, params):
        # pkg/plugin/rpc/types.go ExecuteCommandInput: command, args.
        name = params.get("command", "")
        if name not in self._commands:
            self._send_error(request_id, "unknown command: " + name)
            return _SENT
        result = self._commands[name][1](list(params.get("args") or []))
        if result is not None and not isinstance(result, (list, dict, str)):
            raise TypeError(
                "command %r returned a %s: return a list of dict rows, a dict, a string or None"
                % (name, type(result).__name__)
            )
        # The answer is the record answer, never a bare ok (protocol.md, "Wire
        # Format"; pkg/plugin/rpc/answer_write.go WriteDocumentAnswer): a head,
        # the value as the one record of a `doc` answer, and the terminator. A
        # command that reported nothing writes no record.
        # UNVERIFIED: how the CLI draws a document that is a JSON array or a
        # JSON string. The Go SDK sends the same bytes for a slice or a string.
        head = b"#%d top doc 0: 0:\n" % request_id
        if result is None:
            self._send(head + b"#%d end 0 0 0:\n" % request_id)
            return _SENT
        document = _compact(result)
        if len(document) + 64 > MAX_MESSAGE_SIZE:
            raise ValueError("the answer of command %r is %d bytes, more than one wire message holds" % (name, len(document)))
        # The count is the payload's BYTE count.
        record = b"#%d row %d:" % (request_id, len(document)) + document + b"\n"
        self._send(head + record + b"#%d end 1 0 0:\n" % request_id)
        return _SENT
