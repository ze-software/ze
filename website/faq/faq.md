# Frequently asked questions

These answers cover what Ze does, its release status, and what adopting it involves. If your question is missing, you can ask on [Discord](https://discord.gg/T8s7CjPDne) or open an [issue](https://github.com/ze-software/ze/issues).

<nav class="faq-index" aria-label="FAQ sections">
  <a href="#product" class="cat-meta">
    <span>01</span>
    <strong>Product and readiness</strong>
    <small>What Ze is, what works, and where it fits today.</small>
  </a>
  <a href="#adoption" class="cat-meta">
    <span>02</span>
    <strong>Running and adopting Ze</strong>
    <small>Platforms, migration, configuration, and operations.</small>
  </a>
  <a href="#project" class="cat-meta">
    <span>03</span>
    <strong>Project and support</strong>
    <small>Licensing, contributions, funding, and security.</small>
  </a>
</nav>

<div class="faq-groups">
  <section class="faq-group cat-meta" id="product" aria-labelledby="product-title">
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
      <details class="faq-card cat-meta" id="ready">
        <summary>Is Ze ready for production?</summary>
        <div>
          <p>No. Ze is pre-release software. The BGP engine has automated tests, but production exposure remains limited, and both APIs and configuration syntax can still change.</p>
          <p>A lab is a useful place to try Ze today, whether you want to build a route server, migrate an ExaBGP configuration, or test it against FRR or BIRD. Reports of failures help us find what those tests miss. The <a href="../project/roadmap/">roadmap</a> lists the release work, while the <a href="../quality/">quality pages</a> describe the available tests and their limits.</p>
        </div>
      </details>
      <details class="faq-card cat-meta" id="today">
        <summary>What can Ze actually do today?</summary>
        <div>
          <p>Ze has a broad BGP implementation, YANG-modeled configuration with commit and rollback, Linux interface and FIB management, an SSH CLI, a web interface, APIs, observability, and a plugin system.</p>
          <p>OSPF, IS-IS, MPLS, firewall, VPN, PPPoE, and L2TP code also exists, but those areas remain experimental. Use the <a href="../features/">feature catalog</a> for current status instead of treating this answer as a compatibility matrix.</p>
        </div>
      </details>
      <details class="faq-card cat-meta" id="compare">
        <summary>Why would I use Ze instead of BIRD, FRR, or GoBGP?</summary>
        <div>
          <p>A mature daemon is the better choice when you need an established production platform today. Ze may be worth evaluating if you want a shared configuration model across its CLI, web interface, and APIs, or if its plugins and ExaBGP migration tools fit your deployment. Its <a href="../quality/">quality pages</a> also make the test methods and evidence available for review.</p>
          <p>Each subsystem supplies a YANG model that defines its configuration and supplies the CLI, validation, web editor, reference pages, and MCP tools. The <a href="../compare/">comparison pages</a> describe the differences between Ze and other projects, including features Ze still lacks.</p>
        </div>
      </details>
    </div>
  </section>

  <section class="faq-group cat-meta" id="adoption" aria-labelledby="adoption-title">
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
      <details class="faq-card cat-automate" id="exabgp">
        <summary>I run ExaBGP. Can I move to Ze?</summary>
        <div>
          <p>Yes. Ze includes <code>ze exabgp migrate</code> for configuration conversion and a compatibility bridge that lets existing ExaBGP process scripts run while you port them.</p>
          <p>The <a href="../use-cases/exabgp-migration/">ExaBGP migration guide</a> covers conversion, known differences, and the point where a native Ze plugin becomes the better choice.</p>
        </div>
      </details>
      <details class="faq-card cat-platform" id="daemon-appliance">
        <summary>The daemon or the appliance: which should I run?</summary>
        <div>
          <p>Start with the daemon. It fits into Linux infrastructure you already supervise and is easier to inspect while Ze is still pre-release.</p>
          <p>The appliance packages the same binary and configuration into a read-only system with automatic supervision, no package manager, and no interactive shell. Use it when you want a purpose-built router image rather than another managed Linux host.</p>
        </div>
      </details>
      <details class="faq-card cat-operate" id="config-stability">
        <summary>Will my configuration keep working as Ze changes?</summary>
        <div>
          <p>Configuration syntax can change before the first release, and changes are described in the <a href="../project/changes/">change log</a>. An existing configuration may need editing when you update Ze. Silent reinterpretation is a bug.</p>
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

  <section class="faq-group cat-meta" id="project" aria-labelledby="project-title">
    <header class="faq-group-head">
      <span aria-hidden="true" data-number="03"></span>
      <div>
        <h2 id="project-title">Project and support</h2>
        <p>How the project is licensed, maintained, and supported.</p>
      </div>
    </header>
    <div class="faq-list">
      <details class="faq-card cat-meta" id="license">
        <summary>What does the AGPLv3 license mean for me?</summary>
        <div>
          <p>You can run, inspect, modify, and redistribute Ze under the <a href="../license/">GNU Affero General Public License v3</a>. Running an unmodified Ze for your own network does not require publishing your configuration.</p>
          <p>If you modify Ze and let users interact with that modified version over a network, the AGPL requires you to offer those users the corresponding source. Read the license itself when the distinction matters to a deployment.</p>
        </div>
      </details>
      <details class="faq-card cat-meta" id="contribute-cla">
        <summary>What does the contributor agreement grant?</summary>
        <div>
          <p>A signed-off commit (<code>git commit -s</code>) confirms agreement to the <a href="../contribute/">Contributor License Agreement</a>. You keep your copyright.</p>
          <p>The agreement lets the maintainer offer Ze under additional license terms, including a commercial license. Every contribution remains available to the public under AGPLv3. The <a href="../contribute/guide/">contributor guide</a> covers the submission process.</p>
        </div>
      </details>
      <details class="faq-card cat-meta" id="funding">
        <summary>Who builds Ze, and how is it funded?</summary>
        <div>
          <p>Thomas Mangin develops Ze, with engineering time supported by <a href="https://exa.net.uk">Exa Networks</a>. The ISP has backed the work since it began with ExaBGP in 2009.</p>
          <p>Ze currently has no subscription, paid support tier, or separate commercial entity. The <a href="../contribute/">contribute page</a> explains how the project is maintained and how to help.</p>
        </div>
      </details>
      <details class="faq-card cat-meta" id="ai-development">
        <summary>How is AI used to develop Ze?</summary>
        <div>
          <p>AI coding agents write Ze's code under a process we developed to guide their work and check the result. We agree on a spec before implementation, write tests first, and ask a separate agent to review the change. Commit gates check the repository's rules. Keeping the instructions beside the code gives each new session the same requirements. <a href="../project/ai-development/">How Ze is developed with AI</a> explains how this process developed.</p>
          <p>As with human development, the process is iterative, and the first version of something is often not fully correct. Proving a feature takes many passes and a lot of time and compute. For an RFC, every requirement has to be found, tested, and shown to fail when the code is broken, and that coverage keeps growing after the first implementation is published. The <a href="../quality/rfc-compliance/">RFC compliance pages</a> show what is proven and what is still missing.</p>
          <p>The website is harder to check automatically. An RFC can define behavior precisely enough for a test, while free text leaves more room for unclear explanations and mistakes. Most of the site's figures, feature lists, and references are generated from the code, but the prose still needs review and is likely to contain more mistakes than the code. We revise it as we find them. If you spot a mistake on the site or in Ze, we would appreciate a report on <a href="https://discord.gg/T8s7CjPDne">Discord</a> or the <a href="https://github.com/ze-software/ze/issues">issue tracker</a>.</p>
        </div>
      </details>
      <details class="faq-card cat-meta" id="help">
        <summary>How do I get help or report a security problem?</summary>
        <div>
          <p>Use <a href="https://discord.gg/T8s7CjPDne">Discord</a> for discussion and the <a href="https://github.com/ze-software/ze/issues">issue tracker</a> for reproducible bugs and feature requests.</p>
          <p>Follow the <a href="../security/">security policy</a> for anything security-sensitive. Report vulnerabilities privately instead of opening a public issue.</p>
        </div>
      </details>
    </div>
  </section>
</div>
