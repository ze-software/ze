# Frequently asked questions

Straight answers about what Ze does, where it is ready, and what adopting it involves. Ask on [Discord](https://discord.gg/T8s7CjPDne) or open an [issue](https://github.com/ze-software/ze/issues) when your question is missing.

<nav class="faq-index" aria-label="FAQ sections">
  <a href="#product" class="cat-operate">
    <span>01</span>
    <strong>Product and readiness</strong>
    <small>What Ze is, what works, and where it fits today.</small>
  </a>
  <a href="#adoption" class="cat-routing">
    <span>02</span>
    <strong>Running and adopting Ze</strong>
    <small>Platforms, migration, configuration, and operations.</small>
  </a>
  <a href="#project" class="cat-automate">
    <span>03</span>
    <strong>Project and support</strong>
    <small>Licensing, contributions, funding, and security.</small>
  </a>
</nav>

<div class="faq-groups">
  <section class="faq-group cat-operate" id="product" aria-labelledby="product-title">
    <header class="faq-group-head">
      <span aria-hidden="true" data-number="01"></span>
      <div>
        <h2 id="product-title">Product and readiness</h2>
        <p>Start here if you are deciding whether Ze belongs in your network.</p>
      </div>
    </header>
    <div class="faq-list">
      <details class="faq-card cat-operate" id="what-is-ze" open>
        <summary>What is Ze?</summary>
        <div>
          <p>Ze is an open-source configuration and protocol engine. The repository also builds a Linux network operating system around that engine. It speaks BGP, manages network interfaces, programs the FIB, and serves one configuration over SSH, a web UI, API, and MCP.</p>
          <p>The core is a protocol-agnostic supervisor with an event bus, configuration provider, and plugin manager. Each subsystem registers its own YANG model. Ze uses that model to derive the CLI, completion, validation, web editor, generated reference, and MCP tools.</p>
        </div>
      </details>

      <details class="faq-card cat-routing" id="ready">
        <summary>Is Ze ready for production?</summary>
        <div>
          <p>No. Ze is pre-release software. The BGP engine is substantial and heavily tested, but production exposure remains limited, and both APIs and configuration syntax can still change.</p>
          <p>Use Ze in a lab today. Build a route server, migrate an ExaBGP configuration, run the interop labs against FRR or BIRD, and report failures. The <a href="../project/roadmap/">roadmap</a> lists the release work, while the <a href="../quality/">quality pages</a> show the evidence already available.</p>
        </div>
      </details>

      <details class="faq-card cat-observe" id="today">
        <summary>What can Ze actually do today?</summary>
        <div>
          <p>Ze has a broad BGP implementation, YANG-modeled configuration with commit and rollback, Linux interface and FIB management, an SSH CLI, a web interface, APIs, observability, and a plugin system.</p>
          <p>OSPF, IS-IS, MPLS, firewall, VPN, PPPoE, and L2TP code also exists, but those areas remain experimental. Use the <a href="../features/">feature catalog</a> for current status instead of treating this answer as a compatibility matrix.</p>
        </div>
      </details>

      <details class="faq-card cat-platform" id="compare">
        <summary>Why would I use Ze instead of BIRD, FRR, or GoBGP?</summary>
        <div>
          <p>Choose a mature daemon when you need an established production platform today. Ze is worth evaluating when you want its configuration model, generated operator surfaces, plugin architecture, ExaBGP migration path, or built-in evidence tooling.</p>
          <p>The core remains protocol-agnostic, and each subsystem brings its own YANG model. One schema drives configuration, validation, CLI, web, generated reference, and MCP. The <a href="../compare/">comparison pages</a> show where Ze leads, where it differs, and where the mature projects remain ahead.</p>
        </div>
      </details>
    </div>
  </section>

  <section class="faq-group cat-routing" id="adoption" aria-labelledby="adoption-title">
    <header class="faq-group-head">
      <span aria-hidden="true" data-number="02"></span>
      <div>
        <h2 id="adoption-title">Running and adopting Ze</h2>
        <p>Practical questions about trying Ze before you commit a network to it.</p>
      </div>
    </header>
    <div class="faq-list">
      <details class="faq-card cat-platform" id="platforms">
        <summary>Where does Ze run?</summary>
        <div>
          <p>The daemon runs on Linux under systemd or another process supervisor. Development builds work on macOS and Linux with the supported Go toolchain. Windows is not a supported development or runtime platform.</p>
          <p>Start with the <a href="../guides/quickstart/">quickstart</a>. Use the <a href="../labs/appliance-install/">appliance lab</a> when you want an immutable boot image for a VM or dedicated hardware.</p>
        </div>
      </details>

      <details class="faq-card cat-services" id="exabgp">
        <summary>I run ExaBGP. Can I move to Ze?</summary>
        <div>
          <p>Yes. Ze includes <code>ze config migrate</code> for configuration conversion and a compatibility bridge that lets existing ExaBGP process scripts run while you port them.</p>
          <p>The <a href="../use-cases/exabgp-migration/">ExaBGP migration guide</a> covers conversion, known differences, and the point where a native Ze plugin becomes the better choice.</p>
        </div>
      </details>

      <details class="faq-card cat-operate" id="daemon-appliance">
        <summary>The daemon or the appliance: which should I run?</summary>
        <div>
          <p>Start with the daemon. It fits into Linux infrastructure you already supervise and is easier to inspect while Ze is still pre-release.</p>
          <p>The appliance packages the same binary and configuration into a read-only system with automatic supervision, no package manager, and no interactive shell. Use it when you want a purpose-built router image rather than another managed Linux host.</p>
        </div>
      </details>

      <details class="faq-card cat-routing" id="config-stability">
        <summary>Will my configuration keep working as Ze changes?</summary>
        <div>
          <p>Treat configuration syntax as unstable until the first release. Breaking changes belong in the <a href="../project/changes/">change log</a> and should arrive with an automatic migration or a clear error. Silent reinterpretation is a bug.</p>
          <p>Configuration stability is an explicit milestone on the <a href="../project/roadmap/">roadmap</a>.</p>
        </div>
      </details>

      <details class="faq-card cat-automate" id="ai-mcp">
        <summary>Does Ze need an LLM to run?</summary>
        <div>
          <p>No. Ze runs without an AI service. MCP is an optional operator interface generated from the same schemas and command catalog as the CLI and web interface. It lets an authorized assistant discover and operate the capabilities compiled into a running daemon.</p>
        </div>
      </details>
    </div>
  </section>

  <section class="faq-group cat-automate" id="project" aria-labelledby="project-title">
    <header class="faq-group-head">
      <span aria-hidden="true" data-number="03"></span>
      <div>
        <h2 id="project-title">Project and support</h2>
        <p>How the project is licensed, maintained, and supported.</p>
      </div>
    </header>
    <div class="faq-list">
      <details class="faq-card cat-secure" id="license">
        <summary>What does the AGPLv3 license mean for me?</summary>
        <div>
          <p>You can run, inspect, modify, and redistribute Ze under the <a href="../license/">GNU Affero General Public License v3</a>. Running an unmodified Ze for your own network does not require publishing your configuration.</p>
          <p>If you modify Ze and let users interact with that modified version over a network, the AGPL requires you to offer those users the corresponding source. Read the license itself when the distinction matters to a deployment.</p>
        </div>
      </details>

      <details class="faq-card cat-automate" id="contribute-cla">
        <summary>What does the contributor agreement grant?</summary>
        <div>
          <p>A signed-off commit (<code>git commit -s</code>) confirms agreement to the <a href="../contribute/">Contributor License Agreement</a>. You keep your copyright.</p>
          <p>The agreement lets the maintainer offer Ze under additional license terms, including a commercial license. Every contribution remains available to the public under AGPLv3. The <a href="../contribute/guide/">contributor guide</a> covers the submission process.</p>
        </div>
      </details>

      <details class="faq-card cat-services" id="funding">
        <summary>Who builds Ze, and how is it funded?</summary>
        <div>
          <p>Thomas Mangin develops Ze, with engineering time supported by <a href="https://exa.net.uk">Exa Networks</a>. The ISP has backed the work since it began with ExaBGP in 2009.</p>
          <p>Ze currently has no subscription, paid support tier, or separate commercial entity. The <a href="../contribute/">contribute page</a> explains how the project is maintained and how to help.</p>
        </div>
      </details>

      <details class="faq-card cat-secure" id="help">
        <summary>How do I get help or report a security problem?</summary>
        <div>
          <p>Use <a href="https://discord.gg/T8s7CjPDne">Discord</a> for discussion and the <a href="https://github.com/ze-software/ze/issues">issue tracker</a> for reproducible bugs and feature requests.</p>
          <p>Follow the <a href="../security/">security policy</a> for anything security-sensitive. Report vulnerabilities privately instead of opening a public issue.</p>
        </div>
      </details>
    </div>
  </section>
</div>
