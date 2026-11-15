// Design: website/AI.md -- the homepage's authored copy, as one template
// Related: home.go renders it; the data files supply the cards and the numbers.
package site

// homeTemplate is the homepage under <main>, as its author wrote it.
//
// The hero, the proof block, the why-Ze cards and the quickstart terminal are
// bespoke homepage copy: there is nothing repeated here to
// model as data, and the retired renderer held them as a literal template for
// the same reason. The placeholders are the parts that ARE data.
// home.go fills each one:
//
//	{hero_demo}                    the recorded terminal the homepage replays
//	{hero_demo_file} {hero_demo_title}
//	{hero_demo_transcript}         its file name, manifest title and transcript link
//	{tour_demo_href}               the browser showcase's demonstration link
//	{web_demo_poster} {web_demo_transcript}
//	                              verified browser screenshot and transcript
//	{run_cards}                    website/data/audience.json Run entries, in order
//	{interop_other_row}            nonzero scenarios outside named protocol rows
//	{category_links}               the feature count under each category
//	{feature_overview}             every feature destination, by category
//	{blog_teaser_cards}            the three newest weekly updates
//
// The proof strip's figures are not slots: each is a {{ze:<dotted.key>}} token
// naming its key in data/site-facts.json, which home.go resolves to the marked
// value before it fills the slots.
//
// It is markup rather than a Go builder because it is prose an editor changes,
// and because shell.go already holds themeBootstrap and structuredData the same
// way.
const homeTemplate = `        <div class="site-home">
            <section class="hero" aria-labelledby="hero-title">
                <div>
                    <aside class="hero-start-panel" aria-label="Start with Ze">
                        <div class="hero-start-intro hero-grid">
                            <div class="hero-start-copy hero-copy">
                                <p class="eyebrow">Ze, an OpenNOS</p>
                                <h1 id="hero-title" class="hero-start-title">A network OS.<br /><em>A platform for immutable services.</em></h1>
                                <p class="hero-start-points" aria-label="Key capabilities">
                                    <span>Self-contained</span>
                                    <span>Extend with plugins</span>
                                    <span>Appliance option</span>
                                    <span>ExaBGP compatible</span>
                                </p>
                                <p class="hero-intro">Ze is built on a <a href="architecture/">configuration and plugin engine</a>, with <a href="reference/plugins/">pre-installed plugins</a> for <a href="guides/bgp-peering/">BGP</a>, <a href="guides/ospf/">OSPF</a>, <a href="guides/isis/">IS-IS</a>, <a href="guides/ipsec/">VPNs</a>, <a href="features/interfaces/">interfaces</a>, <a href="features/dns-resolver/">DNS</a>, <a href="reference/plugins/dhcpserver/">DHCP</a> and <a href="features/">other network functions</a>. Its single <a href="reference/configuration/">YANG-defined configuration</a> can be managed through Ze’s own <a href="features/cli-commands/">SSH server, CLI</a>, <a href="features/web-interface/">web interface</a>, <a href="features/mcp-integration/">MCP</a> and <a href="features/api-commands/">APIs</a>. You can <a href="developers/plugins/">develop your own plugins in any programming language</a> and run Ze as a <a href="guides/ubuntu-build-install/">Linux daemon</a> or an <a href="guides/appliance/">immutable OS</a>.</p>
                                <p class="hero-project-intro"><a href="project/why-ze/">Why Ze exists →</a> · Ask questions and follow development on <a href="https://discord.gg/T8s7CjPDne" target="_blank" rel="noopener">Discord</a>.</p>
                                <div class="hero-actions">
                                    <a class="button primary" href="guides/quickstart/">Try the BGP quickstart <span aria-hidden="true">↗</span></a>
                                    <a class="button ghost" href="demos/terminal/">Watch the demos <span aria-hidden="true">↗</span></a>
                                </div>
                                <p class="hero-status">Linux service or immutable appliance. <span>No stable release yet.</span></p>
                            </div>
                            <div class="showcase" data-product-showcase>
                                <div class="screen-tabs" role="group" aria-label="Choose a product interface" hidden>
                                    <button type="button" data-product-target="product-terminal" aria-controls="product-terminal" aria-pressed="true">Watch Ze at work</button>
                                    <button type="button" data-product-target="product-routes" aria-controls="product-routes" aria-pressed="false">Inspect routes</button>
                                    <button type="button" data-product-target="product-web" aria-controls="product-web" aria-pressed="false">Web configuration</button>
                                    <button type="button" data-product-target="product-editor" aria-controls="product-editor" aria-pressed="false">CLI editor</button>
                                    <button type="button" data-product-target="product-dashboard" aria-controls="product-dashboard" aria-pressed="false">Monitor peers</button>
                                </div>
                                <div class="product-panels">
                                <figure class="screen product-screen" id="product-terminal" data-product-panel>
                                    <div class="screen-toolbar"><strong>Ze / <span>{hero_demo_file}</span></strong><a href="{hero_demo_transcript}">Read transcript</a></div>
                                    <div class="screen-media" data-cast-poster="43">{hero_demo}</div>
                                    <figcaption class="screen-description">
                                        <strong id="hero-demo-title">{hero_demo_title}</strong>
                                        <p>Watch ExaBGP migration, inspect BGP peers, then configure, diagnose and repair services without restarting Ze. The transcript includes the commands and their output.</p>
                                        <a href="demos/terminal/">Watch the other demos →</a>
                                        <p class="capture-caption">Enlarged preview from the recording. Play shows the full terminal. Use full screen for a closer look.</p>
                                    </figcaption>
                                </figure>
                                <figure class="screen product-screen" id="product-routes" data-product-panel>
                                    <div class="screen-toolbar"><strong>Ze / Looking glass</strong><a href="assets/product-routes.webp" target="_blank" rel="noopener">Open full size ↗</a></div>
                                    <a href="assets/product-routes.webp" target="_blank" rel="noopener"><img src="assets/product-routes.webp" alt="Ze looking glass with prefix, AS path and community filters, a route graph and route table" width="2820" height="1848" /></a>
                                    <figcaption class="screen-description">
                                        <strong>Follow a route through the network.</strong>
                                        <p>Inspect prefixes, AS paths and peers through a read-only BGP interface.</p>
                                        <a href="features/looking-glass/">Explore the looking glass →</a>
                                        <p class="capture-caption">Captured for the June 2026 LINX talk.</p>
                                    </figcaption>
                                </figure>
                                <figure class="screen product-screen" id="product-web" data-product-panel>
                                    <div class="screen-toolbar"><strong>Ze / Web configuration</strong><a href="{web_demo_poster}" target="_blank" rel="noopener">Open full size ↗</a></div>
                                    <a href="{web_demo_poster}" target="_blank" rel="noopener"><img src="{web_demo_poster}" alt="Ze web configuration showing the system identity form with the committed hostname edge-demo" width="1680" height="1008" loading="lazy" /></a>
                                    <figcaption class="screen-description">
                                        <strong>Configure Ze in your browser.</strong>
                                        <p>Edit a setting, review the changes, apply them and check the active value.</p>
                                        <a href="{tour_demo_href}">Explore the web interface →</a>
                                        <p class="capture-caption">Captured after the hostname commit in an isolated local Ze lab. <a href="{web_demo_transcript}">Read the transcript.</a></p>
                                    </figcaption>
                                </figure>
                                <figure class="screen product-screen" id="product-editor" data-product-panel>
                                    <div class="screen-toolbar"><strong>Ze / CLI configuration editor</strong><a href="assets/product-editor.webp" target="_blank" rel="noopener">Open full size ↗</a></div>
                                    <a href="assets/product-editor.webp" target="_blank" rel="noopener"><img src="assets/product-editor.webp" alt="Ze configuration editor showing a transit peer, a pending remote ASN edit and contextual connection completions" width="1440" height="852" loading="lazy" /></a>
                                    <figcaption class="screen-description">
                                        <strong>Edit the configuration with context.</strong>
                                        <p>Review the pending change and explore the settings available at the current configuration path.</p>
                                        <a href="features/bgp-configuration/">Read about configuration →</a>
                                        <p class="capture-caption">Captured from a local build on 23 October 2026 with an isolated lab configuration. The illustrated edit was not committed.</p>
                                    </figcaption>
                                </figure>
                                <figure class="screen product-screen" id="product-dashboard" data-product-panel>
                                    <div class="screen-toolbar"><strong>Ze / CLI dashboard</strong><a href="assets/product-dashboard.webp" target="_blank" rel="noopener">Open full size ↗</a></div>
                                    <a href="assets/product-dashboard.webp" target="_blank" rel="noopener"><img src="assets/product-dashboard.webp" alt="Ze terminal dashboard showing three established BGP peers, uptime and message counters" width="1384" height="811" loading="lazy" /></a>
                                    <figcaption class="screen-description">
                                        <strong>Keep the peers in view.</strong>
                                        <p>Inspect BGP sessions from Ze’s own CLI.</p>
                                        <a href="features/cli-commands/">Explore CLI interaction →</a>
                                        <p class="capture-caption">A frame from the published terminal recording. Explore the recordings in the demo gallery.</p>
                                    </figcaption>
                                </figure>
                                </div>
                            </div>
                        </div>
                        <div class="hero-action-row">
                            <div class="hero-release-callout">
                                <div class="hero-release-details">
                                    <a
                                        class="hero-release-speaker"
                                        href="zeledon/"
                                        aria-label="Meet Zeledon, the Ze mascot"
                                    >
                                        <img
                                            class="hero-mascot"
                                            src="assets/zeledon.svg"
                                            alt="Zeledon, the Ze bird mascot"
                                            width="134"
                                            height="134"
                                        />
                                    </a>
                                    <p>
                                        <strong class="hero-release-title">Planned release</strong>
                                        <span>Initial version</span><br /><span>Q4 2026</span>
                                    </p>
                                </div>
                            </div>
                            <div class="hero-project-metrics" aria-label="Project size and scope">
                                <div class="hero-metrics-grid">
                                    <a class="hero-project-metric" href="project/activity/">
                                        <strong>{{ze:repo.go_source_lines_display}}</strong>
                                        <span>lines of Go source code</span>
                                    </a>
                                    <a class="hero-project-metric" href="project/activity/">
                                        <strong>{{ze:repo.go_test_lines_display}}</strong>
                                        <span>lines of Go test code</span>
                                    </a>
                                    <a class="hero-project-metric" href="project/activity/">
                                        <strong>{{ze:repo.go_vendor_lines_display}}</strong>
                                        <span>lines of third-party Go code</span>
                                    </a>
                                </div>
                            </div>
                        </div>
                        <div class="hero-outcome-links" aria-label="Ze product map">
                            <a href="architecture/">Core shared by all protocols</a>
                            <a href="reference/configuration/">Configuration models in YANG</a>
                            <a href="features/#routing">BGP, interfaces, FIB</a>
                            <a href="features/ai-first/">CLI, SSH, web, API, MCP</a>
                            <a href="reference/plugins/">Compiled or external plugins</a>
                            <a href="license/">AGPLv3 source</a>
                        </div>
                    </aside>
                </div>
            </section>

            <section id="try" class="home-section-panel home-section-panel-try" aria-labelledby="try-title">
                <div class="section-head reveal cat-meta">
                    <h2 id="try-title">Getting started with Ze.</h2>
                    <p>Run the two-peer BGP lab by hand, or ask Claude Code or Codex to run it with Ze’s bundled skill. Both paths start on a Linux lab host with Go and Git.</p>
                </div>
                <div class="terminal-panel reveal">
                    <div class="terminal" aria-label="Quick start commands">
                        <div class="terminal-dots">
                            <span></span><span></span><span></span>
                            <h3 class="terminal-title">By hand</h3>
                        </div>
                        <div class="terminal-body">
                            <pre><span class="term-comment"># build from source</span>
<span class="term-prompt">$</span> <span class="term-cmd">git clone</span> https://github.com/ze-software/ze.git
<span class="term-prompt">$</span> <span class="term-cmd">cd</span> ze && ./ze version

<span class="term-comment"># copy the supplied local lab configuration</span>
<span class="term-prompt">$</span> mkdir -p lab/r1 lab/r2
<span class="term-prompt">$</span> cp examples/quickstart/r1.conf lab/r1/ze.conf
<span class="term-prompt">$</span> cp examples/quickstart/r2.conf lab/r2/ze.conf

<span class="term-comment"># one peer in each terminal</span>
<span class="term-prompt">$</span> ./ze <span class="term-cmd">start</span> lab/r1/ze.conf
<span class="term-prompt">$</span> ./ze <span class="term-cmd">start</span> lab/r2/ze.conf

<span class="term-comment"># inspect received routes from a third terminal</span>
<span class="term-prompt">$</span> ZE_SSH_PASSWORD=lab ./ze <span class="term-cmd">cli</span> --remote 127.0.0.1:2221 --user lab -c "show bgp rib | received"</pre>
                        </div>
                    </div>
                    <div class="terminal" aria-label="AI setup and lab prompt">
                        <div class="terminal-dots">
                            <span></span><span></span><span></span>
                            <h3 class="terminal-title">With AI</h3>
                        </div>
                        <div class="terminal-body">
                            <pre><span class="term-comment"># build from source</span>
<span class="term-prompt">$</span> <span class="term-cmd">git clone</span> https://github.com/ze-software/ze.git
<span class="term-prompt">$</span> <span class="term-cmd">cd</span> ze &amp;&amp; ./ze version

<span class="term-comment"># make ze available to your agent</span>
<span class="term-prompt">$</span> export PATH="$PWD:$PATH"
<span class="term-prompt">$</span> ze <span class="term-cmd">create agent skill</span>
<span class="term-prompt">$</span> ze <span class="term-cmd">show agent skill</span>

<span class="term-comment"># start Claude Code or Codex from this shell</span>
<span class="term-comment"># in Codex, trust Ze's hook with /hooks</span></pre>
                        </div>
                        <div class="terminal-dots">
                            <span class="terminal-title">Then ask your agent</span>
                        </div>
                        <div class="terminal-body">
                            <pre>Use the ze skill and docs/guide/quickstart.md
to run the local two-peer lab.

Validate both configurations, start both peers,
and verify that the session is established.
Check that r1 receives 198.51.100.0/24 and
r2 receives 192.0.2.0/24. Show me the results.

Stop only the lab processes you started.
Keep the logs if a check fails.</pre>
                        </div>
                    </div>
                </div>
            </section>

            <section id="why-ze" class="home-section-panel home-section-panel-management" aria-labelledby="why-title">
                <article class="why-lead">
                    <div class="why-lead-copy">
                        <h2 id="why-title">Why Ze?</h2>
                        <p>
                            Ze brings BGP, Linux interfaces and forwarding-table management
                            together with network services and plugins in one
                            open-source platform. Use the same configuration model
                            and tools in a lab, as a daemon or as an appliance,
                            with routing policy data, reference pages generated from the code and test
                            results available alongside them.
                        </p>
                        <h3>Manage Ze directly.</h3>
                        <p>
                            Ze runs its own SSH server, CLI and configuration editor.
                            Connect directly to configure and operate it,
                            without a host SSH service or shell. The web interface,
                            REST, gRPC, gNMI and MCP expose the same YANG-modeled
                            configuration.
                        </p>
                        <div class="interface-map" aria-label="Ze’s own SSH server, CLI, web interface, APIs and MCP connect to its YANG-modeled configuration">
                            <div><span>Ze SSH + CLI</span><span>Web</span><span>APIs + MCP</span></div>
                            <div class="map-stem" aria-hidden="true">↓</div>
                            <strong>YANG-modeled configuration</strong>
                            <small>Routing · Interfaces · Network services</small>
                        </div>
                        <nav class="why-lead-links" aria-label="Management documentation">
                            <a href="features/web-interface/">See the browser workflow →</a>
                            <a href="features/cli-commands/">Explore CLI interaction →</a>
                        </nav>
                    </div>
                    <figure class="why-demo">
                        <figcaption>
                            <h3>From connection to configuration</h3>
                            <p>Change a setting through Ze’s SSH editor, review the diff, commit it and check the result.</p>
                        </figcaption>
                        <ol class="why-demo-steps" aria-label="SSH walkthrough steps">
                            <li>Connect</li><li>Edit</li><li>Review</li><li>Commit</li>
                        </ol>
                        <div class="why-demo-player" data-cast-start="{management_demo_start}">
                            {management_demo}
                        </div>
                        <p class="why-demo-note">Playback starts at the SSH introduction ({management_demo_start}) in the full recorded session. Use full screen for a closer look.</p>
                        <nav class="why-lead-links" aria-label="SSH walkthrough resources">
                            <a href="{management_demo_guide}">Full recording and quickstart →</a>
                            <a href="{management_demo_transcript}">Read transcript →</a>
                        </nav>
                    </figure>
                </article>

                <div class="cards usp-grid reveal" aria-label="Ze architectural arguments">
                    <article class="card usp-card cat-operate">
                        <span class="cat">Decision</span>
                        <h3><a href="project/why-ze/">Reasons to consider Ze</a></h3>
                        <p>
                            The overview explains the CLI and operator protocols,
                            in-engine IRR filtering, PeeringDB data, looking glass
                            APIs and shared YANG model. It also describes when
                            another NOS may be a better choice.
                        </p>
                    </article>
                    <article class="card usp-card cat-platform">
                        <span class="cat">Runtime</span>
                            <h3><a href="architecture/">A small core shared by each subsystem</a></h3>
                        <p>
                            The core manages background tasks, messages between components,
                            configuration and plugins. BGP and interface management
                            connect to these shared services.
                        </p>
                    </article>
                    <article class="card usp-card cat-routing">
                        <span class="cat">Routing</span>
                        <h3><a href="architecture/">Network OS built on the engine</a></h3>
                        <p>
                            The shipped daemon speaks BGP, manages Linux
                            interfaces, programs the FIB, and serves its
                            configuration through SSH and the web UI.
                        </p>
                    </article>
                    <article class="card usp-card cat-automate">
                        <span class="cat">Modular</span>
                        <h3><a href="reference/plugins/">Extend Ze with plugins</a></h3>
                        <p>
                            Plugins can be compiled Go modules or external
                            processes. Compiled modules load their YANG into the
                            daemon validator; external plugins can expose their
                            model through <code>ze schema</code>.
                        </p>
                    </article>
                </div>
            </section>

            <section id="proof" class="home-proof-block index4-evidence reveal" aria-labelledby="proof-title">
                <div class="index4-evidence-heading">
                    <div><span class="index4-kicker">Evidence for your decision</span><h2 id="proof-title">Tests that catch bugs</h2></div>
                    <p>Go tests, complete operator scenarios, generated inputs and exchanges with other implementations
                        cover different kinds of failure. These figures show the inventory and the evidence behind it.</p>
                </div>
                <div class="index4-role-path">
                    <div><span class="index4-badge">By use case</span><h3>Start with the role you have in mind.</h3>
                        <p>Inspect the requirements and remaining gaps for your network role, alongside the feature maturity information.</p></div>
                    <div class="index4-role-actions"><a class="button" href="quality/rfc-compliance/profiles/">See completion by use case →</a>
                        <a href="quality/">How Ze is tested</a><a href="#why-ze">Watch the configuration demo</a></div>
                </div>
                <div class="index4-evidence-top">
                    <article class="index4-metric-panel" aria-labelledby="index4-rfc-title">
                        <span class="index4-badge concept-evidence">Standards requirements</span>
                        <h3 id="index4-rfc-title">Mandatory RFC requirements proven</h3>
                        <p class="index4-big-figure">{{ze:rfc.proven_percent}}<span class="index4-percent">%</span></p>
                        <p>{{ze:rfc.proven_display}} of {{ze:rfc.must_kept_display}} requirements for Ze or the OS.</p>
                        <div class="index4-small-figures"><div><b>{{ze:rfc.implemented_display}}</b><span>RFCs implemented</span></div>
                            <div><b>{{ze:rfc.not_walked_display}}</b><span>unreviewed requirement lists excluded</span></div></div>
                        <p class="index4-panel-note">Each test identifies the requirement it checks. The ledger names the evidence and missing coverage.</p>
                        <details class="index4-method"><summary>Counting history</summary><p>Until 2026-10-31: all mandatory rows counted, including annotated rows; summary rows excluded.</p></details>
                        <a class="index4-evidence-link" href="quality/rfc-compliance/">Standards requirements and test results →</a>
                    </article>
                    <article class="index4-metric-panel" aria-labelledby="index4-interop-title">
                        <span class="index4-badge concept-evidence">Tests with other implementations</span>
                        <h3 id="index4-interop-title">Interoperability scenarios</h3>
                        <p class="index4-big-figure">{{ze:interop.scenarios}}</p>
                        <p>BGP sessions, routes, policy and recovery, alongside routing, VPN, access and multi-node cases.</p>
                        <div class="index4-small-figures"><div><b>{{ze:interop.suite_count}}</b><span>lab suites</span></div>
                            <div><b>{{ze:interop.target_display}}</b><span>test images listed</span></div></div>
                        <div class="index4-image-list" aria-label="Listed test images">
                            <span>BIRD</span><span>FRR</span><span>GoBGP</span><span>OpenBGPD</span>
                            <span>freeRouter</span><span>RustyBGP</span><span>rustbgpd</span><span>StayRTR</span>
                            <span>keepalived</span><span>dnsmasq</span><span>Kea</span><span>Knot DNS</span><span>Unbound</span>
                        </div>
                        <p class="index4-image-scope">These test images are reused across many scenarios.
                            Other labs and tools use additional images, including pmacct.</p>
                        <p class="index4-panel-note">A scenario can contain many checks, and many scenarios reuse the same peer implementation.
                            These counts describe the inventory, not current passing runs or individual assertions.</p>
                        <a class="index4-evidence-link" href="quality/qemu-interop-release/#docker-interop">Docker scenarios and peer checks →</a>
                    </article>
                </div>
                <section class="index4-scenario-panel" aria-labelledby="index4-scenario-title">
                    <div class="index4-scenario-heading"><div><span class="index4-kicker">The scenarios behind the figure</span><h3 id="index4-scenario-title">Where the {{ze:interop.scenarios}} scenarios are defined.</h3></div>
                        <a href="labs/bgp-interop/">Explore the BGP lab →</a></div>
                    <table class="index4-scenario-table" role="table">
                        <caption>Scenario inventory across {{ze:interop.suite_count}} lab suites. The shared suite is split by primary protocol; the two RADIUS suites are combined.</caption>
                        <thead role="rowgroup"><tr role="row"><th scope="col" role="columnheader">Protocol or test area</th><th scope="col" role="columnheader">Scenarios</th><th scope="col" role="columnheader">What the cases cover</th></tr></thead>
                        <tbody role="rowgroup">
                            <tr role="row"><th scope="row" role="rowheader">BGP</th><td role="cell">{{ze:interop.protocols.bgp}}</td><td role="cell">Route exchange, path attributes, graceful restart, route reflection, route-server and session cases.</td></tr>
                            <tr role="row"><th scope="row" role="rowheader">OSPFv2</th><td role="cell">{{ze:interop.protocols.ospfv2}}</td><td role="cell">Adjacencies, authentication, areas, LSAs, route policy and recovery.</td></tr>
                            <tr role="row"><th scope="row" role="rowheader">OSPFv3</th><td role="cell">{{ze:interop.protocols.ospfv3}}</td><td role="cell">IPv6 adjacencies, address families, IPv4 over OSPFv3, IPsec protection and recovery.</td></tr>
                            <tr role="row"><th scope="row" role="rowheader">IS-IS</th><td role="cell">{{ze:interop.protocols.isis}}</td><td role="cell">Adjacencies, authentication, levels, metrics, route policy and convergence.</td></tr>
                            <tr role="row"><th scope="row" role="rowheader">BFD</th><td role="cell">{{ze:interop.protocols.bfd}}</td><td role="cell">Sessions, link-local peers and simple-password authentication.</td></tr>
                            <tr role="row"><th scope="row" role="rowheader">BMP</th><td role="cell">{{ze:interop.protocols.bmp}}</td><td role="cell">Route monitoring, local-RIB export, receivers and statistics.</td></tr>
                            <tr role="row"><th scope="row" role="rowheader">RPKI / RTR</th><td role="cell">{{ze:interop.protocols.rpki}}</td><td role="cell">Route-origin validation and validator-cache sessions and preference.</td></tr>
                            <tr role="row"><th scope="row" role="rowheader">VRRP</th><td role="cell">{{ze:interop.protocols.vrrp}}</td><td role="cell">Mastership and VRRPv2 address-owner cases.</td></tr>
                            <tr role="row"><th scope="row" role="rowheader">DNS</th><td role="cell">{{ze:interop.protocols.dns}}</td><td role="cell">Resolver forwarding and DNS service for DHCP clients.</td></tr>
                            <tr role="row"><th scope="row" role="rowheader">DHCPv4</th><td role="cell">{{ze:interop.protocols.dhcpv4}}</td><td role="cell">Renewal, rebinding, NAK handling and shared-segment server cases.</td></tr>
                            <tr role="row"><th scope="row" role="rowheader">DHCPv6</th><td role="cell">{{ze:interop.protocols.dhcpv6}}</td><td role="cell">Client confirmation and renewal against Kea.</td></tr>
                            <tr role="row"><th scope="row" role="rowheader">LDP</th><td role="cell">{{ze:interop.protocols.ldp}}</td><td role="cell">Session backoff, unknown TLVs and keepalive expiry.</td></tr>
                            <tr role="row"><th scope="row" role="rowheader">AS112</th><td role="cell">{{ze:interop.protocols.as112}}</td><td role="cell">Anycast route announcements, communities, origin attributes and redistribution.</td></tr>
                            <tr role="row"><th scope="row" role="rowheader">GTSM</th><td role="cell">{{ze:interop.protocols.gtsm}}</td><td role="cell">TTL protection for ICMP errors related to a protected BGP session.</td></tr>
                            <tr role="row"><th scope="row" role="rowheader">IPsec</th><td role="cell">{{ze:interop.protocols.ipsec}}</td><td role="cell">Tunnel negotiation, authentication and recovery scenarios.</td></tr>
                            <tr role="row"><th scope="row" role="rowheader">L2TP</th><td role="cell">{{ze:interop.protocols.l2tp}}</td><td role="cell">Access tunnels, sessions and PPP cases.</td></tr>
                            <tr role="row"><th scope="row" role="rowheader">PPPoE</th><td role="cell">{{ze:interop.protocols.pppoe}}</td><td role="cell">Client and access-session scenarios.</td></tr>
                            <tr role="row"><th scope="row" role="rowheader">RADIUS</th><td role="cell">{{ze:interop.protocols.radius}}</td><td role="cell">Operator login, authorization, subscriber accounting, CoA and disconnect cases.</td></tr>
                            <tr role="row"><th scope="row" role="rowheader">RSVP-TE</th><td role="cell">{{ze:interop.protocols.rsvpte}}</td><td role="cell">Transit, label and signaling scenarios.</td></tr>
                            <tr role="row"><th scope="row" role="rowheader">Multi-node topologies</th><td role="cell">{{ze:interop.protocols.topology}}</td><td role="cell">Links, routes and service interactions between nodes.</td></tr>
{interop_other_row}
                        </tbody>
                        <tfoot role="rowgroup"><tr role="row"><th scope="row" role="rowheader">All scenarios</th><td role="cell">{{ze:interop.scenarios}}</td><td role="cell">Each scenario is counted once by its primary protocol or test area, including cases that exercise several protocols.</td></tr></tfoot>
                    </table>
                </section>
                <div class="index4-test-grid" aria-label="Other test layers">
                    <article><span class="index4-badge concept-evidence">Package behavior</span><h3>Go tests</h3><p class="index4-test-figure">{{ze:tests.unit_display}}</p>
                        <p>Check encoded bytes and parsed values. Verify configuration, state transitions and plugins.
                            Mutation checks with gomu examine whether tests catch code changes.</p><a href="quality/unit-fuzz-mutation/">Go tests and mutation checks →</a></article>
                    <article><span class="index4-badge concept-evidence">Visible behavior</span><h3>End-to-end tests</h3><p class="index4-test-figure">{{ze:tests.e2e_display}}</p>
                        <p>Start sessions and exchange route updates. Run editor commands, commits and reloads.
                            Check output, errors and saved state.</p><a href="quality/functional-ci/">Scenarios and expected results →</a></article>
                    <article><span class="index4-badge concept-evidence">Generated inputs</span><h3>Fuzz targets</h3><p class="index4-test-figure">{{ze:tests.fuzz_display}}</p>
                        <p>Generate inputs for parsers and decoders. Probe packets and configuration text for crashes.
                            Replay saved failures as regression cases.</p><a href="quality/unit-fuzz-mutation/#fuzz-targets-are-still-tests">Fuzz inputs and regression cases →</a></article>
                </div>
                <p class="index4-development-note">Ze is written with AI agents working under a quality process based on ISO management systems.
                    <a href="project/ai-development/">How Ze is developed with AI</a> explains the process.</p>
                <div class="interop-strip" aria-label="Implementations Ze is tested against">
                    <span class="interop-strip-label concept-evidence">Tested against</span>
                    <div class="interop-groups">
                        <span class="interop-group"><b>BGP</b><a class="concept-software" href="quality/qemu-interop-release/#docker-interop">FRR</a><a class="concept-software" href="quality/qemu-interop-release/#docker-interop">BIRD</a><a class="concept-software" href="quality/qemu-interop-release/#docker-interop">GoBGP</a><a class="concept-software" href="quality/qemu-interop-release/#docker-interop">OpenBGPd</a><a class="concept-software" href="quality/qemu-interop-release/#docker-interop">FreeRtr</a><a class="concept-software" href="quality/qemu-interop-release/#docker-interop">RustyBGP</a><a class="concept-software" href="quality/qemu-interop-release/#docker-interop">rustbgpd</a><a class="concept-software" href="features/exabgp-compatibility/">ExaBGP</a></span>
                        <span class="interop-group"><b>BGP support</b><a class="concept-software" href="quality/qemu-interop-release/#docker-interop">StayRTR</a><a class="concept-software" href="quality/qemu-interop-release/#docker-interop">pmacct</a></span>
                        <span class="interop-group"><b>VPN</b><a class="concept-software" href="quality/qemu-interop-release/#docker-interop">strongSwan</a></span>
                        <span class="interop-group"><b>Access</b><a class="concept-software" href="quality/qemu-interop-release/#docker-interop">accel-ppp</a><a class="concept-software" href="quality/qemu-interop-release/#docker-interop">xl2tpd</a><a class="concept-software" href="quality/qemu-interop-release/#docker-interop">FreeRADIUS</a></span>
                        <span class="interop-group"><b>Redundancy</b><a class="concept-software" href="quality/qemu-interop-release/#docker-interop">keepalived</a></span>
                    </div>
                </div>
            </section>

            <section id="features-summary" class="home-section-panel home-section-panel-map" aria-labelledby="core-title">
                <div class="section-head reveal">
                    <h2 id="core-title">Capabilities and current limitations.</h2>
                    <div class="reference-intro">
                        <p>Feature pages explain behavior and limits. The <a href="reference/feature-status/">maturity record</a> separates available code from the evidence still needed before a feature can be recommended for production.</p>
                    </div>
                </div>
                <div class="reference-grid reveal">
                    <a class="card reference-card cat-platform" href="reference/configuration/">
                        <span class="cat">Configuration</span>
                        <strong class="reference-card-title">The YANG tree</strong>
                        <span class="reference-card-description">Settings for the documented build, including plugins →</span>
                    </a>
                    <a class="card reference-card cat-operate" href="reference/cli/">
                        <span class="cat">Commands</span>
                        <strong class="reference-card-title">{{ze:cli_commands}} CLI commands</strong>
                        <span class="reference-card-description">Syntax, arguments and pipe operators →</span>
                    </a>
                    <a class="card reference-card cat-automate" href="reference/plugins/">
                        <span class="cat">Plugins</span>
                        <strong class="reference-card-title">{{ze:plugin_registrations}} registered plugins</strong>
                        <span class="reference-card-description">Purpose, dependencies, configuration roots and source →</span>
                    </a>
                </div>
                <p class="reference-generation-note">The configuration and command references come from the definitions used by Ze itself. When those definitions change, the next website build updates the reference, including settings supplied by plugins.</p>
                <h3 class="feature-directory-title">Explore features by category</h3>
                <div class="legend reveal" role="group" aria-label="Browse features by category">
{category_links}
                </div>
                <div class="capability-grid">
{feature_overview}
                    <aside class="capability capability-guide">
                        <div class="capability-guide-card">
                            <span class="cat">Find the right page</span>
                            <h3>Keep the reference close.</h3>
                            <p>The documentation index groups guides by the job you want to do. The feature catalogue includes maturity filters.</p>
                            <a href="docs/">Open documentation →</a>
                            <a href="features/">Browse the feature catalogue →</a>
                        </div>
                    </aside>
                </div>
            </section>

            <section class="home-section-panel home-section-panel-run index4-try" aria-labelledby="run-title">
                <div class="section-head reveal cat-platform">
                    <h2 id="run-title">Choose how to try Ze.</h2>
                    <p>
                        A local BGP lab is the simplest starting point. If you
                        already run ExaBGP or want a different deployment,
                        follow the path that matches your setup.
                    </p>
                </div>
                <div class="index4-choice-grid reveal">
{run_cards}
                </div>
                <div class="index4-other-paths">
                    <strong>Other ways to evaluate</strong>
                    <a href="guides/netlab/">netlab and containerlab</a>
                    <a href="labs/netlab/">Explore a supplied topology</a>
                    <a href="labs/bgp-interop/">BGP interoperability labs</a>
                    <a href="guides/ai-agent/">Test with an AI agent</a>
                    <a href="reference/cli/">Explore the CLI</a>
                    <a href="guides/public-looking-glass/">Publish a looking glass</a>
                    <a href="guides/ze-install/#choose-a-deployment-path">Compare all deployment paths →</a>
                </div>
                <p class="index4-try-note">The quickstart builds from source and needs Go and Git. It checks the BGP control plane; receiving routes does not establish forwarding connectivity. For a live topology that also checks kernel routes and traffic, use the netlab guide.</p>
            </section>

            <section class="home-section-panel home-section-panel-momentum" aria-labelledby="blog-teaser-title">
                <div class="section-head reveal">
                    <h2 id="blog-teaser-title">Recent engineering notes.</h2>
                    <p>
                        The update reports describe changes recorded in git
                        and shared in Discord's <code>ze-news</code> channel.
                    </p>
                </div>
                <div class="cards reveal">
{blog_teaser_cards}
                </div>
                <div class="link-list reveal">
                    <a href="project/changes/">See all updates</a>
                </div>
            </section>

        </div>
`
