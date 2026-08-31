# Documentation

Pick the path that matches your intent. Start with the card that describes the
job you are trying to do, then follow the links inside that path.

<div class="docs-path-grid cards" aria-label="Documentation paths">
    <article class="card docs-path-card cat-operate">
        <span class="cat">Start</span>
        <h2>Get Ze running</h2>
        <p>Guides that take you from a blank shell to a working setup.</p>
        <div class="link-list">
            <a href="../guides/quickstart/">Quickstart <span>two BGP peers talking in under five minutes</span></a>
            <a href="../guides/ze-install/">Install Ze <span>daemon install or bootable appliance</span></a>
            <a href="../guides/ubuntu-build-install/">Build on Ubuntu <span>compile, install, create zefs, set up SSH</span></a>
            <a href="../features/bgp-configuration/">BGP configuration <span>peer settings, inheritance, validation, and policy</span></a>
            <a href="../features/cli-commands/"><code>CLI</code> commands <span>diff, commit, history, and operator commands</span></a>
            <a href="../guides/cli/">CLI tour <span>interactive, one-shot, pipes, and runtime control</span></a>
            <a href="../guides/lifecycle/">Lifecycle and rollback <span>reload, restart, archive, update, and recover</span></a>
        </div>
    </article>
    <article class="card docs-path-card cat-routing">
        <span class="cat">Operate</span>
        <h2>Do a specific job</h2>
        <p>Task-oriented guides for protocol setup, access services, migration, and diagnostics.</p>
        <div class="link-list">
            <a href="../use-cases/exabgp-migration/">ExaBGP migration <span>convert an existing config and process scripts</span></a>
            <a href="../guides/operator-access-rbac/">SSH and RBAC <span>local users, profiles, and operator access</span></a>
            <a href="../guides/radius/">RADIUS operator login <span>authenticate SSH, web, and MCP users safely</span></a>
            <a href="../guides/flowspec-route-reflector/">FlowSpec route reflector <span>reflect FlowSpec routes to iBGP clients</span></a>
            <a href="../guides/flowspec-protected-router/">FlowSpec protected router <span>turn FlowSpec into nftables protection</span></a>
            <a href="../guides/ddos-mitigation/">DDoS mitigation <span>detect floods and respond locally or upstream</span></a>
            <a href="../guides/anomaly/">Anomaly detection <span>baseline source behaviour and respond shadow-first</span></a>
            <a href="../guides/public-looking-glass/">Looking glass <span>publish read-only BGP visibility</span></a>
            <a href="../guides/firewall/">Firewall and policy routing <span>protect and steer traffic</span></a>
            <a href="../guides/ospf/">OSPF, IS-IS, and static routes <span>bring routing protocols online</span></a>
            <a href="../guides/mpls/">MPLS <span>label switching with LDP and RSVP-TE</span></a>
            <a href="../features/srv6/">SRv6 <span>segment routing over IPv6 and SID programming</span></a>
            <a href="../guides/traffic-control/">Traffic control <span>shape, queue, and prioritise egress traffic</span></a>
            <a href="../guides/pppoe/">PPPoE and L2TP <span>access concentration paths</span></a>
            <a href="../guides/ipsec/">Native IPsec <span>configure IKEv2 initiator and responder tunnels</span></a>
            <a href="../guides/monitoring/">Monitoring and diagnostics <span>flow export, MRT, and production checks</span></a>
            <a href="../guides/debugging-tools/">Debugging tools <span>trace netlink, capture packets, and inspect state</span></a>
            <a href="../guides/bgp-peering/">BGP peering <span>groups, families, capabilities, and verification</span></a>
            <a href="../guides/bgp-policy/">BGP policy <span>import, export, validation, and redistribution</span></a>
            <a href="../guides/bgp-resilience/">BGP resilience <span>refresh, GR, persistence, reflection, and multipath</span></a>
            <a href="../guides/fleet-config/">Fleet Management <span>hub, managed clients, cached config, and reconnect</span></a>
        </div>
    </article>
    <article class="card docs-path-card cat-automate">
        <span class="cat">Automate</span>
        <h2>Use and extend every surface</h2>
        <p>Management transports, browser and public interfaces, and the plugin SDK.</p>
        <div class="link-list">
            <a href="../guides/api/">REST and gRPC <span>authentication, commands, config sessions, and streaming</span></a>
            <a href="../guides/gnmi/">gNMI <span>Capabilities, Get, Set, Subscribe, TLS, and metrics</span></a>
            <a href="../guides/web-interface/">Web interface <span>workbench, editing, commands, and live updates</span></a>
            <a href="../guides/looking-glass/">Looking Glass <span>routes, topology, Birdwatcher API, and security</span></a>
            <a href="../guides/mcp/overview/">MCP <span>tools, resources, authentication, and remote access</span></a>
            <a href="../developers/plugins/">Plugin development <span>SDK, protocol, YANG, handlers, commands, and testing</span></a>
        </div>
    </article>
    <article class="card docs-path-card cat-platform">
        <span class="cat">Reference</span>
        <h2>Look up generated facts</h2>
        <p>Information-oriented pages generated from live data where possible.</p>
        <div class="link-list">
            <a href="../features/">Features <span>capabilities by category and maturity</span></a>
            <a href="../reference/cli/">CLI reference <span>all generated commands in one place</span></a>
            <a href="../reference/configuration/">Configuration reference <span>the whole config as a searchable tree</span></a>
            <a href="../reference/command-equivalents/">Command equivalents <span>Ze syntax across Junos, IOS XR, SR OS, and VyOS</span></a>
            <a href="../reference/rfcs/">RFC status <span>implemented RFCs, partial support, and remaining gaps</span></a>
            <a href="../reference/plugins/">Plugin catalogue <span>runtime plugins, dependencies, and config roots</span></a>
            <a href="../reference/glossary/">Glossary <span>routing, policy, configuration, and testing terms</span></a>
            <a href="../reference/deprecations/">Deprecated options <span>removed syntax and its replacements</span></a>
            <a href="../reference/dependencies/">Dependencies <span>direct Go packages and why they exist</span></a>
        </div>
    </article>
    <article class="card docs-path-card cat-automate">
        <span class="cat">Design</span>
        <h2>Understand the system</h2>
        <p>Explanation pages for architecture, comparisons, performance, labs, and deployment shapes.</p>
        <div class="link-list">
            <a href="../architecture/">Architecture <span>engine, config, plugins, and operator surfaces</span></a>
            <a href="../architecture/route-selection/">Route selection <span>how Ze picks the best path, step by step</span></a>
            <a href="../compare/">Compare <span>Ze against BIRD, FRR, GoBGP, and others</span></a>
            <a href="../performance/">Performance <span>measured BGP benchmarks and methodology</span></a>
            <a href="../architecture/vpp-deployment/">VPP deployment reference <span>startup.conf, NIC, LCP, and notes</span></a>
            <a href="../labs/">Labs and usage examples <span>interop proof and deployment shapes</span></a>
            <a href="../guides/chaos-testing/">Chaos testing <span>deterministic faults, properties, replay, and shrink</span></a>
            <a href="../use-cases/route-server/">IXP route server <span>member policy, validation, and replay checks</span></a>
            <a href="../use-cases/transit-edge-rpki/">Transit edge with RPKI <span>dual transit, origin validation, and failover</span></a>
            <a href="../use-cases/flowspec-injection/">FlowSpec injection <span>authorised, atomic, and reversible route control</span></a>
        </div>
    </article>
    <article class="card docs-path-card cat-secure">
        <span class="cat">Community</span>
        <h2>Keep up and contribute</h2>
        <p>Project context for people deciding whether to try Ze or help shape it.</p>
        <div class="link-list">
            <a href="../project/why-ze/">Why Ze <span>what makes Ze different and who it is for</span></a>
            <a href="../project/roadmap/">Roadmap <span>the path to the first release</span></a>
            <a href="../project/changes/">Changes <span>what shipped, week by week</span></a>
            <a href="../blog/">Blog <span>longer notes behind the work</span></a>
            <a href="../faq/">FAQ <span>answers before you commit time</span></a>
            <a href="../contribute/">Contribute <span>code, bug reports, and interop results</span></a>
            <a href="../contribute/developer-setup/">Developer setup <span>build, test, and debug the Ze source tree</span></a>
            <a href="../contribute/rfc-implementation-guide/">RFC implementation guide <span>how a new RFC gets built and proven</span></a>
            <a href="../contribute/documentation-testing/">Documentation testing <span>keep docs honest with tested command output</span></a>
            <a href="../project/history/">Project history <span>how ExaBGP's programmable model grew into Ze</span></a>
            <a href="../contribute/testing/">Contributor testing <span>choose the right proof layer and run it locally</span></a>
        </div>
    </article>
</div>
