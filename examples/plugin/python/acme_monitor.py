"""acme-monitor: an example external Ze plugin written in Python.

It brings its own YANG config module, is told its config at start and on each
commit, and serves one command. Run it as `python3 acme_monitor.py`.
"""

from ze_plugin import ConfigError, Plugin

plugin = Plugin("acme-monitor")

# The module name ends in -conf, so its container joins the config. The
# presence statement lets a config with no acme-monitor section load: without
# it, the mandatory leaf would be missed.
plugin.schema("""module acme-monitor-conf {
    namespace "urn:acme:monitor:conf";
    prefix acme;

    description "Configuration of the acme monitor, an external plugin.";

    revision 2026-11-16 {
        description "Initial revision";
    }

    container acme-monitor {
        presence "The acme monitor is configured";
        description "Settings of the acme monitor.";

        leaf endpoint {
            type string;
            mandatory true;
            description "The URL the monitor probes.";
        }

        leaf interval {
            type uint8 {
                range "1..10";
            }
            default 5;
            description "Seconds between two probes.";
        }
    }
}
""")

# What the monitor runs with. An empty endpoint means it is not configured.
state = {"endpoint": "", "interval": 5}


@plugin.config("acme-monitor")
class Monitor:
    def verify(self, new):
        endpoint = new.get("endpoint", "")
        if endpoint.startswith("http://"):
            raise ConfigError(
                "acme-monitor endpoint %s uses plain HTTP, and the monitor probes over HTTPS only" % endpoint,
                fix="set acme-monitor endpoint https://" + endpoint[len("http://"):],
            )
        new.int("interval", default=5)

    def apply(self, new, changed):
        self._store(new)
        plugin.log("acme-monitor: applied endpoint=%s interval=%d changed=%s"
                   % (state["endpoint"] or "(none)", state["interval"], ",".join(sorted(changed))))

    def rollback(self, old):
        self._store(old)
        plugin.log("acme-monitor: rolled back to endpoint=%s interval=%d"
                   % (state["endpoint"] or "(none)", state["interval"]))

    def _store(self, config):
        # An empty section means it was removed: the monitor stops probing.
        state["endpoint"] = config.get("endpoint", "")
        state["interval"] = config.int("interval", default=5)


@plugin.command("show acme-monitor status", help="Show the endpoint and the interval the acme monitor runs with")
def status(args):
    return [{"endpoint": state["endpoint"], "interval": state["interval"], "configured": bool(state["endpoint"])}]


plugin.run()
