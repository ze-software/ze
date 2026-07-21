# Documentation

Pick the path that matches your intent. Start with the card that describes the
job you are trying to do, then follow the links inside that path.

<div class="docs-path-grid cards" aria-label="Documentation paths">
    <article class="card docs-path-card cat-operate">
        <span class="cat">Start</span>
        <h2>Get Ze running</h2>
        <p>Learning-oriented pages that take you from a blank shell to a working setup.</p>
        <div class="link-list">
            <a href="guide/quickstart/">Quickstart <span>two BGP peers talking in under five minutes</span></a>
            <a href="guide/ze-install/">Install Ze <span>daemon install or bootable appliance</span></a>
            <a href="guide/ubuntu-build-install/">Build on Ubuntu <span>compile, install, create zefs, set up SSH</span></a>
            <a href="features/configuration/">Configuration <span>the YANG model Ze uses everywhere</span></a>
            <a href="features/cli-commands/"><code>CLI</code> commands <span>diff, commit, history, and operator commands</span></a>
            <a href="guide/cli/">CLI tour <span>interactive, one-shot, pipes, and runtime control</span></a>
            <a href="guide/lifecycle/">Lifecycle and rollback <span>reload, restart, archive, update, and recover</span></a>
        </div>
    </article>
    <article class="card docs-path-card cat-routing">
        <span class="cat">Operate</span>
        <h2>Do a specific job</h2>
        <p>Task-oriented guides for protocol setup, access services, migration, and diagnostics.</p>
        <div class="link-list">
            <a href="../usage/exabgp-migration/">ExaBGP migration <span>convert an existing config and process scripts</span></a>
            <a href="guide/operator-access-rbac/">SSH and RBAC <span>local users, profiles, and operator access</span></a>
            <a href="guide/radius/">RADIUS operator login <span>authenticate SSH, web, and MCP users safely</span></a>
            <a href="guide/flowspec-route-reflector/">FlowSpec route reflector <span>reflect FlowSpec routes to iBGP clients</span></a>
            <a href="guide/flowspec-protected-router/">FlowSpec protected router <span>turn FlowSpec into nftables protection</span></a>
            <a href="guide/ddos-mitigation/">DDoS mitigation <span>detect floods and respond locally or upstream</span></a>
            <a href="guide/anomaly/">Anomaly detection <span>baseline source behaviour and respond shadow-first</span></a>
            <a href="guide/looking-glass-howto/">Looking glass <span>publish read-only BGP visibility</span></a>
            <a href="guide/firewall/">Firewall and policy routing <span>protect and steer traffic</span></a>
            <a href="guide/ospf/">OSPF, IS-IS, and static routes <span>bring routing protocols online</span></a>
            <a href="guide/pppoe/">PPPoE and L2TP <span>access concentration paths</span></a>
            <a href="guide/ipsec/">Native IPsec <span>configure IKEv2 initiator and responder tunnels</span></a>
            <a href="guide/monitoring/">Monitoring and diagnostics <span>flow export, MRT, and production checks</span></a>
            <a href="guide/bgp-peering/">BGP peering <span>groups, families, capabilities, and verification</span></a>
            <a href="guide/bgp-policy/">BGP policy <span>import, export, validation, and redistribution</span></a>
            <a href="guide/bgp-resilience/">BGP resilience <span>refresh, GR, persistence, reflection, and multipath</span></a>
            <a href="guide/fleet-config/">Fleet Management <span>hub, managed clients, cached config, and reconnect</span></a>
        </div>
    </article>
    <article class="card docs-path-card cat-automate">
        <span class="cat">Automate</span>
        <h2>Use and extend every surface</h2>
        <p>Management transports, browser and public interfaces, and the plugin SDK.</p>
        <div class="link-list">
            <a href="guide/api/">REST and gRPC <span>authentication, commands, config sessions, and streaming</span></a>
            <a href="guide/gnmi/">gNMI <span>Capabilities, Get, Set, Subscribe, TLS, and metrics</span></a>
            <a href="guide/web-interface/">Web interface <span>workbench, editing, commands, and live updates</span></a>
            <a href="guide/looking-glass/">Looking Glass <span>routes, topology, Birdwatcher API, and security</span></a>
            <a href="guide/mcp/overview/">MCP <span>tools, resources, authentication, and remote access</span></a>
            <a href="plugin-development/">Plugin development <span>SDK, protocol, YANG, handlers, commands, and testing</span></a>
        </div>
    </article>
    <article class="card docs-path-card cat-platform">
        <span class="cat">Reference</span>
        <h2>Look up generated facts</h2>
        <p>Information-oriented pages generated from live data where possible.</p>
        <div class="link-list">
            <a href="../features/">Features <span>capabilities by category and maturity</span></a>
            <a href="features/rfc-status/">RFC status <span>implemented RFCs, partial support, and remaining gaps</span></a>
            <a href="../cli/">CLI reference <span>all generated commands in one place</span></a>
            <a href="../config-reference/">Configuration reference <span>the whole config as a searchable tree</span></a>
            <a href="../dependencies/">Dependencies <span>direct Go packages and why they exist</span></a>
        </div>
    </article>
    <article class="card docs-path-card cat-automate">
        <span class="cat">Design</span>
        <h2>Understand the system</h2>
        <p>Explanation pages for architecture, comparisons, performance, labs, and deployment shapes.</p>
        <div class="link-list">
            <a href="architecture/">Architecture <span>engine, config, plugins, and operator surfaces</span></a>
            <a href="../compare/">Compare <span>Ze against BIRD, FRR, GoBGP, and others</span></a>
            <a href="../performance/">Performance <span>measured BGP benchmarks and methodology</span></a>
            <a href="research/vpp-deployment-reference/">VPP deployment reference <span>startup.conf, NIC, LCP, and notes</span></a>
            <a href="../labs/">Labs and usage examples <span>interop proof and deployment shapes</span></a>
            <a href="guide/chaos-testing/">Chaos testing <span>deterministic faults, properties, replay, and shrink</span></a>
            <a href="../usage/route-server/">IXP route server <span>member policy, validation, and replay checks</span></a>
            <a href="../usage/transit-edge-rpki/">Transit edge with RPKI <span>dual transit, origin validation, and failover</span></a>
            <a href="../usage/flowspec-injection/">FlowSpec injection <span>authorised, atomic, and reversible route control</span></a>
        </div>
    </article>
    <article class="card docs-path-card cat-secure">
        <span class="cat">Community</span>
        <h2>Keep up and contribute</h2>
        <p>Project context for people deciding whether to try Ze or help shape it.</p>
        <div class="link-list">
            <a href="../roadmap/">Roadmap <span>the path to the first release</span></a>
            <a href="../changes/">Changes <span>what shipped, week by week</span></a>
            <a href="../blog/">Blog <span>longer notes behind the work</span></a>
            <a href="../faq/">FAQ <span>answers before you commit time</span></a>
            <a href="../contribute/">Contribute <span>code, bug reports, and interop results</span></a>
            <a href="history/">Project history <span>how ExaBGP's programmable model grew into Ze</span></a>
            <a href="glossary/">Glossary <span>BGP, policy, configuration, operator, and testing terms</span></a>
            <a href="contributing/testing/">Contributor testing <span>choose the right proof layer and run it locally</span></a>
        </div>
    </article>
</div>
