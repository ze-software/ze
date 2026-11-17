"""acme-monitor: an example external Ze plugin written in Python.

It brings three YANG modules: its config section, its two commands, and the
rpc that describes the argument of one of them. It is told its config at start
and on each commit, and answers its commands. Run it as
`python3 acme_monitor.py`.

Its config module, acme-monitor-conf, has the leaves of the module of the same
name that Ze's tests declare for their own acme monitor plugin
(internal/test/externalyang/externalyang.go, validText): the same names,
types, bounds, mandatory leaf and presence, with no default. Ze's repository
config sweep judges every config under test/ with that module, so a config
written for this example is valid there too. Change both together.
"""

from ze_plugin import CommandError, ConfigError, Plugin

plugin = Plugin("acme-monitor")

# The config. The module name ends in -conf, so its container joins the
# config schema as the section `acme-monitor`. The container has a presence
# statement, so a config with no acme-monitor section does not miss the
# mandatory target.
plugin.schema("""module acme-monitor-conf {
    namespace "urn:acme:monitor:conf";
    prefix acme;

    description "Configuration of the acme monitor, an external plugin.";

    revision 2026-11-17 {
        description "Initial revision";
    }

    container acme-monitor {
        presence "The acme monitor is configured";
        description "Settings of the acme monitor.";

        leaf target {
            type string {
                length "1..64";
            }
            mandatory true;
            description "The host the monitor probes.";
        }

        leaf interval {
            type uint8 {
                range "1..10";
            }
            description "Seconds between two probes.";
        }
    }
}
""")

# The commands. The module name ends in -cmd. A command is a `config false`
# container that holds ze:command, under a verb container (`show` here). The
# containers on the way are shared with Ze's own commands: only the last one
# is this plugin's. The leaves of a command container are its arguments, and
# Ze checks a value against the leaf's type before it asks the plugin.
plugin.schema("""module acme-monitor-cmd {
    namespace "urn:acme:monitor:cmd";
    prefix acmecmd;
    import ze-extensions { prefix ze; }

    description "Commands of the acme monitor, an external plugin.";

    container show {
        config false;

        container acme-monitor {
            config false;
            ze:help "Inspect the acme monitor.";

            // show acme-monitor status: no argument.
            container status {
                config false;
                ze:command "acme-monitor:show-status";
                ze:help "Show the target and the probe interval.";
            }

            // show acme-monitor target <name>: one argument.
            container target {
                config false;
                ze:command "acme-monitor:show-target";
                // The rpc whose input MCP, REST and gRPC list as the
                // parameters of this command.
                ze:rpc "acme-monitor-api:show-target";
                ze:help "Show the configured target of the acme monitor.";

                leaf name {
                    type string {
                        length "1..16";
                    }
                    mandatory true;
                    ze:help "Name of the target.";
                }
            }
        }
    }
}
""")

# The rpc of `show acme-monitor target`. Its input repeats the argument leaf:
# it is what the API surfaces publish as the command's parameters.
plugin.schema("""module acme-monitor-api {
    namespace "urn:acme:monitor:api";
    prefix acmeapi;
    import ze-extensions { prefix ze; }

    description "The rpcs of the acme monitor commands, an external plugin.";

    rpc show-target {
        ze:help "Show the configured target of the acme monitor.";
        input {
            leaf name {
                type string {
                    length "1..16";
                }
                mandatory true;
                ze:help "Name of the target.";
            }
        }
    }
}
""")

# What the monitor runs with: the host it probes, and the seconds between two
# probes when the config states them. None is a section that is absent, or an
# interval the config does not state: the module has no default.
state = {"target": None, "interval": None}


@plugin.config("acme-monitor")
class Monitor:
    def verify(self, new):
        if not new:
            return
        target = new.get("target", "")
        if "://" in target:
            host = target.split("://", 1)[1].split("/", 1)[0]
            raise ConfigError(
                "acme-monitor target %s is a URL, and the monitor probes a host name" % target,
                fix="set acme-monitor target %s" % host,
            )
        new.int("interval", default=None)

    def apply(self, new, changed):
        self._store(new)
        plugin.log("acme-monitor: applied target=%s interval=%s changed=%s"
                   % (state["target"] or "(none)", _interval_text(), ",".join(sorted(changed))))

    def rollback(self, old):
        self._store(old)
        plugin.log("acme-monitor: rolled back to target=%s interval=%s"
                   % (state["target"] or "(none)", _interval_text()))

    def _store(self, config):
        # An empty section means it was removed: the monitor stops probing.
        if not config:
            state["target"] = None
            state["interval"] = None
            return
        state["target"] = config.get("target")
        state["interval"] = config.int("interval", default=None)


def _interval_text():
    """The interval as the answers and the log write it."""
    if state["interval"] is None:
        return "not set"
    return "%d" % state["interval"]


# The command names are the paths of the containers acme-monitor-cmd defines.
@plugin.command("show acme-monitor status")
def status(args):
    if state["target"] is None:
        raise CommandError("acme-monitor is not configured, so it probes nothing",
                           fix="set acme-monitor target <host>")
    return {"target": state["target"], "interval": _interval_text()}


@plugin.command("show acme-monitor target")
def target(args):
    # Ze checked the value against the leaf `name` before it asked: args holds
    # that one value, from 1 to 16 characters.
    if not args:
        raise CommandError("show acme-monitor target names no target", fix="show acme-monitor target <name>")
    name = args[0]
    plugin.log("acme-monitor: asked for target %s" % name)
    if name != state["target"]:
        configured = state["target"] or "none"
        raise CommandError("acme-monitor has no target %s (configured: %s)" % (name, configured),
                           fix="show acme-monitor status")
    return {"target": name, "interval": _interval_text()}


plugin.run()
