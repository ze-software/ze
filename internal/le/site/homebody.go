// Design: website/AI.md -- the homepage's authored copy, as one template
// Related: home.go renders it; the data files supply the cards and the numbers.
package site

// homeTemplate is the homepage under <main>, as its author wrote it.
//
// The hero, the proof block, the why-Ze cards, the quickstart terminal and the
// try-safely panel are bespoke homepage copy: there is nothing repeated here to
// model as data, and the retired renderer held them as a literal template for
// the same reason. The eleven placeholders are the parts that ARE data, and
// home.go fills each one:
//
//	{hero_demo}                    the recorded terminal the hero replays
//	{whats_new}                    the newest article, the newest week, the note
//	{unit_tests} {e2e_tests}       the proof strip's six data-ze-stat spans
//	{fuzz_targets} {rfc_enrolled}
//	{rfc_must_checks} {interop_targets}
//	{run_cards} {who_cards}        website/data/audience.json, in its own order
//	{category_links}               the feature count under each category
//	{blog_teaser_cards}            the three newest weekly updates
//
// It is markup rather than a Go builder because it is prose an editor changes,
// and because shell.go already holds themeBootstrap and structuredData the same
// way.
const homeTemplate = `            <section class="hero" aria-labelledby="hero-title">
                <div>
                    <aside class="hero-start-panel" aria-label="Start with Ze">
                        <div class="hero-start-intro">
                            <div class="hero-start-copy">
                                <h1 id="hero-title" class="hero-start-title">Ze, an OpenNOS</h1>
                                <p class="hero-start-points" aria-label="Key capabilities">
                                    <span>Plugin extensible</span>
                                    <span>Appliance option</span>
                                    <span>ExaBGP compatible</span>
                                </p>
                                <p class="hero-start-lead">
                                    Ze speaks <a href="guides/bgp-peering/"><strong>BGP</strong></a>,
                                    <a href="guides/isis/"><strong>ISIS</strong></a>,
                                    <a href="guides/ospf/"><strong>OSPF</strong></a>,
                                    manages <a href="features/interfaces/"><strong>interfaces</strong></a> and tunnels
                                    (<a href="guides/ipsec/"><strong>IPsec VPN</strong></a>, <a href="features/interfaces/#wireguard-configuration"><strong>WireGuard</strong></a>),
                                    programs the <a href="reference/plugins/fib-kernel/"><strong>FIB</strong></a>, and exposes one
                                    <a href="features/bgp-configuration/"><strong>YANG</strong></a>-modeled configuration
                                    through builtin SSH, a <a href="features/web-interface/"><strong>web</strong></a>
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
                            </div>
                            <figure class="hero-product-demo" aria-labelledby="hero-demo-title">
                                <div class="hero-product-frame">
                                    <div class="hero-product-bar" aria-hidden="true">
                                        <span class="terminal-demo__dots"><i></i><i></i><i></i></span>
                                        <span>cli-dashboard.terminal</span>
                                        <span>CAST</span>
                                    </div>
                                    {hero_demo}
                                </div>
                                <figcaption>
                                    <strong id="hero-demo-title">Live BGP dashboard</strong>
                                    <span>Replayable Ze terminal lab</span>
                                    <a href="demos/terminal/#live-bgp-dashboard">Read transcript</a>
                                </figcaption>
                            </figure>
                        </div>
                        <div class="hero-action-row">
                            <nav class="hero-primary-actions" aria-label="Start with Ze">
                                <a class="hero-start-action" href="project/why-ze/">
                                    <strong>Why Ze exists</strong>
                                    <small>Learn how Ze came to be.</small>
                                </a>
                                <a class="hero-start-action" href="demos/terminal/#web-configuration-commit">
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
                        <h2 id="proof-title">Release claims stay checkable.</h2>
                        <p>
                            Every homepage number links to the page where you can
                            inspect the test layer, transcript, peer list, RFC
                            gate, or generated source evidence behind it.
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
                        <span class="proof-check">Local test, fuzz, and mutation evidence.</span>
                    </a>
                    <a class="proof" href="quality/rfc-compliance/">
                        <strong
                            >{rfc_must_checks} <span class="label">RFC MUST checks</span></strong
                        >
                        <ul>
                            <li>{rfc_enrolled} RFCs inspected</li>
                            <li>Gaps disclosed before claims</li>
                            <li>Tests tied to requirement IDs</li>
                        </ul>
                        <span class="proof-check">RFC requirement ledger.</span>
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
                        <span class="proof-check">Functional transcript format and rerun path.</span>
                    </a>
                    <a class="proof" href="quality/unit-fuzz-mutation/#fuzz-targets-are-still-tests">
                        <strong>{fuzz_targets} <span class="label">fuzz targets</span></strong>
                        <ul>
                            <li>Parsers, external inputs</li>
                            <li>Wire formats, config files</li>
                            <li>Saved crashes become regression cases</li>
                        </ul>
                        <span class="proof-check">Fuzz crashes kept as regression cases.</span>
                    </a>
                    <a class="proof" href="quality/qemu-interop-release/#docker-interop">
                        <strong
                            >{interop_targets} <span class="label">interop targets</span></strong
                        >
                        <ul>
                            <li>Real third-party daemons</li>
                            <li>BGP sessions in Docker</li>
                            <li>Routes checked by peer CLIs</li>
                        </ul>
                        <span class="proof-check">Docker interop peer list.</span>
                    </a>
                </div>
                <div class="interop-strip" aria-label="Tested BGP peer implementations">
                    <span class="interop-strip-label">Tested against routing stacks</span>
                    <a href="quality/qemu-interop-release/#docker-interop">FRR</a>
                    <a href="quality/qemu-interop-release/#docker-interop">BIRD</a>
                    <a href="quality/qemu-interop-release/#docker-interop">GoBGP</a>
                    <a href="quality/qemu-interop-release/#docker-interop">OpenBGPd</a>
                    <a href="quality/qemu-interop-release/#docker-interop">FreeRtr</a>
                    <a href="quality/qemu-interop-release/#docker-interop">RustyBGP</a>
                    <a href="quality/qemu-interop-release/#docker-interop">rustbgpd</a>
                    <a href="features/exabgp-compatibility/">ExaBGP migration path</a>
                </div>
            </section>


            <section id="why-ze" class="home-section-panel home-section-panel-why" aria-labelledby="why-title">
                <div class="section-head reveal tone-sky">
                    <h2 id="why-title"><a href="project/why-ze/">Why Ze?</a></h2>
                    <p>
                        Ze is a network operating system, and more: a routing
                        daemon, appliance runtime, lab router, and protocol
                        engine that brings policy data, generated references,
                        APIs, MCP tools, and product evidence into one operator
                        workflow.
                    </p>
                </div>

                <div class="cards usp-grid reveal" aria-label="Ze architectural arguments">
                    <article class="card usp-card tone-mint">
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
                    <article class="card usp-card tone-pink">
                        <span class="cat">Honest</span>
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
            <section class="home-section-panel home-section-panel-run" aria-labelledby="run-title">
                <div class="section-head reveal tone-teal">
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

            <section id="features-summary" class="home-section-panel home-section-panel-map" aria-labelledby="core-title">
                <div class="section-head reveal">
                    <h2 id="core-title">Generated references are part of the product.</h2>
                    <p>Read the generated references before you run Ze.</p>
                </div>
                <div class="section-note reveal">
                    <p>
                        Ze is an open-source configuration and protocol engine.
                        The network operating system built on it speaks BGP,
                        manages Linux interfaces, programs the FIB, and serves
                        the same YANG-modeled configuration through SSH, web,
                        API, and MCP.
                    </p>
                </div>
                <div class="legend reveal" role="group" aria-label="Browse features by category">
{category_links}
                </div>
            </section>

            <section id="try" class="home-section-panel home-section-panel-try" aria-labelledby="try-title">
                <div class="section-head reveal tone-grape">
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
`
