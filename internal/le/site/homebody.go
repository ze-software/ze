// Design: website/AI.md -- the homepage's authored copy, as one template
// Related: home.go renders it; the data files supply the cards and the numbers.
package site

// homeTemplate is the homepage under <main>, as its author wrote it.
//
// The hero, the proof block, the why-Ze cards, the quickstart terminal and the
// try-safely panel are bespoke homepage copy: there is nothing repeated here to
// model as data, and the retired renderer held them as a literal template for
// the same reason. The placeholders are the parts that ARE data.
// home.go fills each one:
//
//	{hero_demo}                    the recorded terminal the homepage replays
//	{hero_demo_file} {hero_demo_title}
//	{hero_demo_transcript}         its file name, manifest title and transcript link
//	{tour_demo_href}               the recording "Watch a demo" opens
//	{web_demo_poster} {web_demo_video} {web_demo_transcript}
//	                              verified browser media from that recording
//	{whats_new}                    the newest article, the newest week, the note
//	{unit_tests} {e2e_tests}       the proof strip's nine data-ze-stat spans
//	{fuzz_targets} {interop_targets}
//	{rfc_proven} {rfc_proven_percent}
//	{rfc_gated_implemented}
//	{rfc_implemented} {rfc_inspected}
//	{run_cards} {who_cards}        website/data/audience.json, in its own order
//	{category_links}               the feature count under each category
//	{feature_overview}             every feature destination, by category
//	{blog_teaser_cards}            the three newest weekly updates
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
                                    <span>Plugin extensible</span>
                                    <span>Appliance option</span>
                                    <span>ExaBGP compatible</span>
                                </p>
                                <p class="hero-intro">Extend Ze with plugins for networking, automation and services of your own.</p>
                                <p class="hero-start-lead">
                                    Ze speaks <a href="guides/bgp-peering/"><strong>BGP</strong></a>,
                                    <a href="guides/isis/"><strong>ISIS</strong></a>,
                                    <a href="guides/ospf/"><strong>OSPF</strong></a>,
                                    manages <a href="features/interfaces/"><strong>interfaces</strong></a> and tunnels
                                    (<a href="guides/ipsec/"><strong>IPsec VPN</strong></a>, <a href="features/interfaces/#wireguard-configuration"><strong>WireGuard</strong></a>),
                                    programs the <a href="reference/plugins/fib-kernel/"><strong>FIB</strong></a>, and exposes one
                                    <a href="features/bgp-configuration/"><strong>YANG</strong></a>-modeled configuration
                                    through its own built-in SSH server, a <a href="features/web-interface/"><strong>web</strong></a>
                                    interface, <a href="features/api-commands/"><strong>APIs</strong></a>
                                    (<a href="guides/api/#rest-endpoints"><strong>REST</strong></a>, <a href="guides/api/#grpc-services"><strong>gRPC</strong></a>,
                                    <a href="guides/gnmi/"><strong>gNMI</strong></a>), <a href="features/cli-commands/"><strong>CLI</strong></a>, and
                                    <a href="features/mcp-integration/"><strong>MCP</strong></a>.
                                </p>
                                <p class="hero-start-lead">
                                    It is built on a configuration and protocol
                                    engine that runs as a <a href="architecture/"><strong>daemon</strong></a> or
                                    <a href="guides/appliance/"><strong>immutable appliance</strong></a>.
                                </p>
                                <div class="hero-actions">
                                    <a class="button primary" href="demos/terminal/">Watch the demos <span aria-hidden="true">↗</span></a>
                                    <a class="button ghost" href="guides/quickstart/">Try the BGP quickstart <span aria-hidden="true">↗</span></a>
                                </div>
                            </div>
                            <div class="showcase" data-product-showcase>
                                <div class="screen-tabs" role="group" aria-label="Choose a product interface" hidden>
                                    <button type="button" data-product-target="product-routes" aria-controls="product-routes" aria-pressed="true">Looking glass</button>
                                    <button type="button" data-product-target="product-web" aria-controls="product-web" aria-pressed="false">Web configuration</button>
                                    <button type="button" data-product-target="product-editor" aria-controls="product-editor" aria-pressed="false">CLI editor</button>
                                    <button type="button" data-product-target="product-dashboard" aria-controls="product-dashboard" aria-pressed="false">CLI dashboard</button>
                                </div>
                                <div class="product-panels">
                                <figure class="screen product-screen" id="product-routes" data-product-panel>
                                    <div class="screen-toolbar"><strong>Ze / Looking glass</strong><a href="assets/product-routes.png" target="_blank" rel="noopener">Open full size ↗</a></div>
                                    <a href="assets/product-routes.png" target="_blank" rel="noopener"><img src="assets/product-routes.png" alt="Ze looking glass with prefix, AS path and community filters, a route graph and route table" width="2820" height="1848" /></a>
                                    <figcaption class="screen-description">
                                        <strong>Follow a route through the network.</strong>
                                        <p>Inspect prefixes, AS paths and peers through a read-only BGP interface.</p>
                                        <a href="features/looking-glass/">Explore the looking glass →</a>
                                        <p class="capture-caption">Captured for the June 2026 LINX talk.</p>
                                    </figcaption>
                                </figure>
                                <figure class="screen product-screen" id="product-web" data-product-panel>
                                    <div class="screen-toolbar"><strong>Ze / Web configuration</strong><a href="{web_demo_poster}" target="_blank" rel="noopener">Open full size ↗</a></div>
                                    <div class="screen-media">
                                        <video controls playsinline preload="none" poster="{web_demo_poster}" aria-label="Edit, review and commit configuration in Ze's web interface">
                                            <source src="{web_demo_video}" type="video/webm" />
                                            <a href="{web_demo_video}">Download the browser recording</a>.
                                        </video>
                                    </div>
                                    <figcaption class="screen-description">
                                        <strong>Configure Ze in your browser.</strong>
                                        <p>Edit a setting, review its diff, commit the draft and verify the active value.</p>
                                        <a href="{tour_demo_href}">Explore the web interface →</a>
                                        <p class="capture-caption">A real session with an isolated local Ze lab. <a href="{web_demo_transcript}">Read the transcript.</a></p>
                                    </figcaption>
                                </figure>
                                <figure class="screen product-screen" id="product-editor" data-product-panel>
                                    <div class="screen-toolbar"><strong>Ze / CLI configuration editor</strong><a href="assets/product-editor.jpg" target="_blank" rel="noopener">Open full size ↗</a></div>
                                    <a href="assets/product-editor.jpg" target="_blank" rel="noopener"><img src="assets/product-editor.jpg" alt="Ze configuration editor showing a transit peer, a pending remote ASN edit and contextual connection completions" width="1440" height="852" loading="lazy" /></a>
                                    <figcaption class="screen-description">
                                        <strong>Edit the configuration with context.</strong>
                                        <p>Review the pending change and explore the settings available at the current configuration path.</p>
                                        <a href="features/bgp-configuration/">Read about configuration →</a>
                                        <p class="capture-caption">Captured from a local build on 23 October 2026 with an isolated lab configuration. The illustrated edit was not committed.</p>
                                    </figcaption>
                                </figure>
                                <figure class="screen product-screen" id="product-dashboard" data-product-panel>
                                    <div class="screen-toolbar"><strong>Ze / CLI dashboard</strong><a href="assets/product-dashboard.png" target="_blank" rel="noopener">Open full size ↗</a></div>
                                    <a href="assets/product-dashboard.png" target="_blank" rel="noopener"><img src="assets/product-dashboard.png" alt="Ze terminal dashboard showing three established BGP peers, uptime and message counters" width="1384" height="811" loading="lazy" /></a>
                                    <figcaption class="screen-description">
                                        <strong>Keep the peers in view.</strong>
                                        <p>Inspect BGP sessions from Ze’s own CLI.</p>
                                        <a href="features/cli-commands/">Explore CLI interaction →</a>
                                        <p class="capture-caption">A frame from the published terminal recording. Replay the demonstration below.</p>
                                    </figcaption>
                                </figure>
                                </div>
                            </div>
                        </div>
                        <div class="hero-action-row">
                            <nav class="hero-primary-actions" aria-label="Start with Ze">
                                <a class="hero-start-action" href="project/why-ze/">
                                    <strong>Why Ze exists</strong>
                                    <small>Learn how Ze came to be.</small>
                                </a>
                                <a class="hero-start-action" href="{tour_demo_href}">
                                    <strong>Watch a demo</strong>
                                    <small>Discover the web interface.</small>
                                </a>
                                <a
                                    class="hero-start-action search-trigger"
                                    href="search/"
                                    aria-expanded="false"
                                    ><strong>Search the site</strong
                                    ><small>Docs and commands. <span class="search-shortcut-hint" aria-hidden="true"><kbd>⌘K</kbd><span>or</span><kbd>⌘/</kbd></span></small
                                    ></a
                                >
                                <a
                                    class="hero-start-action"
                                    href="https://discord.gg/T8s7CjPDne"
                                    target="_blank"
                                    rel="noopener"
                                    ><strong>Join Discord</strong
                                    ><small>Ask questions, get help.</small
                                    ></a
                                >
                            </nav>
                            <div class="hero-release-callout">
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
                                    Expected initial release: Q4 2026.<br />
                                    No stable release yet.
                                </p>
                            </div>
                        </div>
                        <div class="hero-outcome-links" aria-label="Ze product map">
                            <a href="architecture/">Protocol-agnostic core</a>
                            <a href="reference/configuration/">YANG per subsystem</a>
                            <a href="features/#routing">BGP, interfaces, FIB</a>
                            <a href="features/ai-first/">CLI, SSH, web, API, MCP</a>
                            <a href="reference/plugins/">Compiled or external plugins</a>
                            <a href="license/">AGPLv3 source</a>
                        </div>
                    </aside>
                </div>
            </section>

{whats_new}

            <section id="proof" class="home-proof-block reveal" aria-labelledby="proof-title">
                <div class="home-proof-head">
                    <div>
                        <h2 id="proof-title">Code quality</h2>
                        <p>
                            Explore the tests behind Ze: unit tests, end-to-end
                            scenarios, protocol checks, fuzzing and interoperability
                            with other implementations. Each figure links to the
                            evidence and its current coverage.
                        </p>
                    </div>
                    <div class="home-proof-actions">
                        <a class="button" href="quality/">Read the evidence map</a>
                        <a class="button" href="demos/terminal/">Watch product demos</a>
                    </div>
                </div>
                <div class="proof-strip" aria-label="Project evidence">
                    <a class="proof" href="quality/unit-fuzz-mutation/">
                        <strong
                            >{unit_tests} <span class="label">unit tests</span></strong
                        >
                        <ul>
                            <li>Wire encoding, parsing</li>
                            <li>Config, FSM, plugins</li>
                            <li>gomu mutates code to check assertions</li>
                        </ul>
                        <span class="proof-check concept-evidence">Local test, fuzz, and mutation evidence.</span>
                    </a>
                    <a class="proof" href="quality/functional-ci/">
                        <strong
                            >{e2e_tests}
                            <span class="label">end to end tests</span></strong
                        >
                        <ul>
                            <li>Peering, sessions, updates</li>
                            <li>Editor, commits, reloads</li>
                            <li>Commands checked as operators run them</li>
                        </ul>
                        <span class="proof-check concept-evidence">Functional transcript format and rerun path.</span>
                    </a>
                    <a class="proof" href="quality/rfc-compliance/">
                        <strong
                            >{rfc_implemented}
                            <span class="label">RFCs supported</span></strong
                        >
                        <ul>
                            <li>{rfc_gated_implemented} MUSTs to test</li>
                            <li>{rfc_proven} checked with a test</li>
                            <li>{rfc_proven_percent}% done</li>
                        </ul>
                        <span class="proof-check concept-evidence">RFC requirement ledger.</span>
                    </a>
                    <a class="proof" href="quality/unit-fuzz-mutation/#fuzz-targets-are-still-tests">
                        <strong>{fuzz_targets} <span class="label">fuzz targets</span></strong>
                        <ul>
                            <li>Parsers, external inputs</li>
                            <li>Wire formats, config files</li>
                            <li>Saved crashes become regression cases</li>
                        </ul>
                        <span class="proof-check concept-evidence">Fuzz crashes kept as regression cases.</span>
                    </a>
                    <a class="proof" href="quality/qemu-interop-release/#docker-interop">
                        <strong
                            >{interop_targets} <span class="label">interop targets</span></strong
                        >
                        <ul>
                            <li>BGP, OSPF, IS-IS, BFD, BMP, RPKI, VRRP</li>
                            <li>IPsec, L2TP, PPPoE, RADIUS</li>
                            <li>Real daemons in Docker, checked by peer CLIs</li>
                        </ul>
                        <span class="proof-check concept-evidence">Docker interop peer list.</span>
                    </a>
                </div>
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

            <section id="why-ze" class="home-section-panel home-section-panel-management" aria-labelledby="why-title">
                <article class="why-lead">
                    <div class="why-lead-copy">
                        <h2 id="why-title">Why Ze?</h2>
                        <p>
                            Ze brings BGP, Linux interfaces and FIB programming
                            together with network services and plugins in one
                            open-source platform. Use the same configuration model
                            and tools in a lab, as a daemon or as an appliance,
                            with policy data, generated references and test
                            evidence available alongside them.
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
                            Start here for the selling points: powerful CLI,
                            operator protocols, in-engine IRR filtering,
                            PeeringDB data, looking glass APIs, one YANG model,
                            and when to choose another NOS instead.
                        </p>
                    </article>
                    <article class="card usp-card cat-platform">
                        <span class="cat">Runtime</span>
                        <h3><a href="architecture/">Small core, registered subsystems</a></h3>
                        <p>
                            The core holds the supervisor, message bus, config
                            provider, and plugin manager. BGP and interface
                            management register into it.
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
                        <h3><a href="reference/plugins/">Plugins keep their own contract</a></h3>
                        <p>
                            Plugins can be compiled Go modules or external
                            processes. Compiled modules load their YANG into the
                            daemon validator; external plugins can expose their
                            model through <code>ze schema</code>.
                        </p>
                    </article>
                </div>
            </section>
            <section id="features-summary" class="home-section-panel home-section-panel-map" aria-labelledby="core-title">
                <div class="section-head reveal">
                    <h2 id="core-title">Documentation that matches the code.</h2>
                    <div class="reference-intro">
                        <p>Read the generated references before you run Ze.</p>
                        <p>The same YANG schema and command definitions drive Ze and its references. Each website build brings changes into the documentation, including configuration supplied by plugins.</p>
                    </div>
                </div>
                <div class="reference-grid reveal">
                    <article class="card reference-card cat-platform">
                        <span class="cat">Configuration</span>
                        <h3>The full YANG tree</h3>
                        <p>Browse every configuration setting in the documented build, including the settings supplied by plugins.</p>
                        <a class="audience-card-cta" href="reference/configuration/">Configuration reference</a>
                    </article>
                    <article class="card reference-card cat-operate">
                        <span class="cat">Commands</span>
                        <h3>Command syntax and help</h3>
                        <p>Find commands, arguments and pipe operators from the same definitions that supply Ze's built-in help.</p>
                        <a class="audience-card-cta" href="reference/cli/">CLI reference</a>
                    </article>
                    <article class="card reference-card cat-automate">
                        <span class="cat">Plugins</span>
                        <h3>Plugins and dependencies</h3>
                        <p>Explore registered plugins, their dependencies, configuration roots and the source that implements them.</p>
                        <a class="audience-card-cta" href="reference/plugins/">Plugin catalogue</a>
                    </article>
                </div>
                <h3 class="feature-directory-title">Explore features by category</h3>
                <div class="legend reveal" role="group" aria-label="Browse features by category">
{category_links}
                </div>
                <p class="capability-note">Open a feature for its behavior and limitations, and <a href="reference/feature-status/">check its maturity</a> before choosing a deployment.</p>
                <div class="capability-grid">
{feature_overview}
                </div>
            </section>

            <section class="home-section-panel home-section-panel-run" aria-labelledby="run-title">
                <div class="section-head reveal cat-platform">
                    <h2 id="run-title">
                        Run it as a lab,<br />daemon, or appliance.
                    </h2>
                    <p>
                        The same binary and configuration support each path,
                        from a <a href="labs/netlab/">netlab topology</a>
                        or BGP interop lab to spare hardware.
                    </p>
                </div>
                <div class="audience run-path-grid reveal">
{run_cards}
                </div>
            </section>

            <section class="home-section-panel home-section-panel-demo" aria-labelledby="recording-title">
                <div class="section-head">
                    <h2 id="recording-title">Watch Ze at work.</h2>
                    <p>The screenshots introduce the interfaces. This recording shows the terminal in use, with its transcript available alongside it.</p>
                </div>
                <figure class="hero-product-demo" aria-labelledby="hero-demo-title">
                    <div class="hero-product-frame">
                        <div class="hero-product-bar" aria-hidden="true">
                            <span class="terminal-demo__dots"><i></i><i></i><i></i></span>
                            <span>{hero_demo_file}</span>
                            <span>CAST</span>
                        </div>
                        {hero_demo}
                    </div>
                    <figcaption>
                        <strong id="hero-demo-title">{hero_demo_title}</strong>
                        <span>Replayable Ze terminal lab</span>
                        <a href="{hero_demo_transcript}">Read transcript</a>
                    </figcaption>
                </figure>
            </section>
            <section class="home-section-panel home-section-panel-usecases" aria-labelledby="audience-title">
                <div class="section-head reveal">
                    <h2 id="audience-title">Safe ways to try Ze before the first release.</h2>
                    <p>
                        Ze is early enough that routing feedback can still
                        change the system. These cards give each reader a
                        low-risk starting point.
                    </p>
                </div>
                <div class="audience home-usecase-grid reveal">
{who_cards}
                </div>
            </section>

            <section id="try" class="home-section-panel home-section-panel-try" aria-labelledby="try-title">
                <div class="section-head reveal cat-meta">
                    <h2 id="try-title">First paths for routing feedback.</h2>
                    <p>The BGP lab, ExaBGP migration, and appliance install are good starting points.</p>
                </div>
                <div class="terminal-panel reveal">
                    <div class="terminal" aria-label="Quick start commands">
                        <div class="terminal-dots">
                            <span></span><span></span><span></span>
                            <span class="terminal-title">quickstart.sh</span>
                        </div>
                        <div class="terminal-body">
                            <pre><span class="term-comment"># build from source</span>
<span class="term-prompt">$</span> <span class="term-cmd">git clone</span> https://github.com/ze-software/ze.git
<span class="term-prompt">$</span> <span class="term-cmd">cd</span> ze && <span class="term-cmd">make</span> build

<span class="term-comment"># set up credentials and configure</span>
<span class="term-prompt">$</span> bin/ze <span class="term-cmd">init</span>
<span class="term-prompt">$</span> bin/ze <span class="term-cmd">config import</span> router.conf

<span class="term-comment"># start</span>
<span class="term-prompt">$</span> bin/ze <span class="term-cmd">start</span>

<span class="term-comment"># from another terminal</span>
<span class="term-prompt">$</span> bin/ze <span class="term-cmd">cli</span> -c "show bgp peer list"
<span class="term-prompt">$</span> bin/ze <span class="term-cmd">cli</span> -c "monitor event"</pre>
                        </div>
                    </div>
                    <div class="terminal-note">
                        <span class="tag">Good starting points</span>
                        <p>
                            A lab peer, a migrated ExaBGP config, or a
                            looking-glass instance can produce useful reports
                            from people who know routing operations.
                        </p>
                        <div class="link-list">
                            <a href="labs/bgp-interop/"
                                >BGP interop lab
                                <span>FRR, BIRD, and GoBGP in Docker</span></a
                            >
                            <a href="use-cases/exabgp-migration/"
                                >ExaBGP migration
                                <span>try an existing config and process script</span></a
                            >
                            <a href="guides/appliance/"
                                >Appliance install
                                <span>ISO media, PXE provisioning, spare hardware</span></a
                            >
                            <a href="guides/public-looking-glass/"
                                >Looking glass
                                <span>publish read-only BGP visibility</span></a
                            >
                            <a href="features/ai-first/"
                                >AI-assisted operations
                                <span>MCP exposes Ze commands to tools</span></a
                            >
                        </div>
                    </div>
                </div>
            </section>

            <section class="home-section-panel home-section-panel-momentum" aria-labelledby="blog-teaser-title">
                <div class="section-head reveal">
                    <h2 id="blog-teaser-title">Recent engineering notes.</h2>
                    <p>
                        Weekly updates come from git history and Discord's
                        <code>ze-news</code>. They stay specific and technical.
                    </p>
                </div>
                <div class="cards reveal">
{blog_teaser_cards}
                </div>
                <div class="link-list reveal">
                    <a href="project/changes/">See all updates</a>
                </div>
            </section>

            <section id="try-safely" class="home-section-panel home-section-panel-safe" aria-labelledby="try-safely-title">
                <div class="status-panel try-safely-panel reveal">
                    <div class="status-copy">
                        <span class="tag">Try safely</span>
                        <h2 id="try-safely-title">Try Ze before the first release.</h2>
                        <p>
                            Start where a mistake cannot affect a live network.
                        </p>
                        <div class="actions">
                            <a class="button primary" href="labs/bgp-interop/">Run a BGP lab</a>
                            <a class="button" href="guides/quickstart/">Read the quickstart</a>
                        </div>
                    </div>
                    <div class="status-table" aria-label="Safe starting points">
                        <div class="status-row">
                            <strong>Release</strong>
                            <span
                                >Expected Q4 2026. No stable release has shipped
                                yet, and configuration may change.</span
                            >
                        </div>
                        <div class="status-row">
                            <strong>BGP lab</strong>
                            <span
                                >Exercise Ze against FRR, BIRD, and GoBGP
                                without touching a production router.</span
                            >
                        </div>
                        <div class="status-row">
                            <strong>Migration</strong>
                            <span
                                >Try the ExaBGP migration path against an
                                existing config before changing automation.</span
                            >
                        </div>
                        <div class="status-row">
                            <strong>Appliance</strong>
                            <span
                                >Boot Ze on spare hardware with the same binary
                                and configuration model as daemon mode.</span
                            >
                        </div>
                        <div class="status-row">
                            <strong>Source</strong>
                            <span
                                >Read the code, generated docs, RFC gate, and
                                test evidence before deciding where Ze belongs.</span
                            >
                        </div>
                    </div>
                </div>
            </section>
        </div>
`
