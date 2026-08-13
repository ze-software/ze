# Frequently asked questions

The questions people tend to ask before they spend time on Ze. If yours is not here, ask on [Discord](https://discord.gg/T8s7CjPDne) or open an [issue](https://github.com/ze-software/ze/issues).

<div class="faq-index" aria-label="FAQ topics">
  <a href="#what-is-ze" class="cat-operate"><span>01</span><strong>What Ze is</strong></a>
  <a href="#ready" class="cat-routing"><span>02</span><strong>Readiness</strong></a>
  <a href="#compare" class="cat-platform"><span>03</span><strong>Why another NOS</strong></a>
  <a href="#contribute-cla" class="cat-automate"><span>04</span><strong>Contributing</strong></a>
  <a href="#help" class="cat-secure"><span>05</span><strong>Help and security</strong></a>
</div>

<div class="faq-list">
  <details class="faq-card cat-operate" id="what-is-ze" open>
    <summary>What is Ze?</summary>
    <div>
      <p>Ze is an open-source configuration and protocol engine. The network operating system built on it speaks BGP, manages Linux network interfaces, programs the FIB, and serves its configuration over SSH, a web UI, API, and MCP.</p>
      <p>That operating surface is not hardwired into the core. The core holds a message bus, a config provider and a plugin manager; BGP, interface management and other subsystems register themselves with their own YANG and extend the config tree where they belong.</p>
      <p>Once a subsystem declares its model, Ze derives the CLI, completion, validation, web editor and MCP tools from the schema.</p>
    </div>
  </details>

  <details class="faq-card cat-routing" id="ready" open>
    <summary>Is Ze ready for production?</summary>
    <div>
      <p>Not yet, and the site says so everywhere on purpose. The routing core is heavily tested, but production exposure is still limited and the configuration syntax can still change before the first release.</p>
      <p>The right place for Ze today is a lab: build a route server, migrate an ExaBGP config, stand up a looking glass, run the interop labs against real FRR or BIRD, and report where it breaks. The <a href="../roadmap/">roadmap</a> explains what remains before a stable release.</p>
    </div>
  </details>

  <details class="faq-card cat-platform" id="compare">
    <summary>Why would I use Ze instead of BIRD, FRR, or GoBGP?</summary>
    <div>
      <p>Those projects are mature, and Ze does not pretend otherwise. The <a href="../compare/">comparison page</a> is blunt about where they are still ahead.</p>
      <p>Ze's difference is the model: the core stays protocol-agnostic, and subsystems bring YANG with them. One schema feeds the CLI, validation, web editor, generated references and MCP tools, while plugins extend the daemon without a second operator surface.</p>
    </div>
  </details>

  <details class="faq-card cat-observe" id="today">
    <summary>What can Ze actually do today?</summary>
    <div>
      <p>The BGP implementation is broad: IPv4 and IPv6 unicast, labeled unicast, VPNv4 and VPNv6, EVPN, FlowSpec, add-path, graceful restart in several flavours, RPKI, route reflection, and a growing list of families and capabilities.</p>
      <p>OSPFv2, OSPFv3, IS-IS, and MPLS are in the core. Around that sit a firewall, VPN, PPPoE and L2TP access concentration, flow export, and appliance packaging. The <a href="../features/">features page</a> marks each capability by status, and the <a href="../compare/">comparison page</a> puts every protocol feature next to the other daemons.</p>
    </div>
  </details>

  <details class="faq-card cat-services" id="exabgp">
    <summary>I run ExaBGP. Can I move to Ze?</summary>
    <div>
      <p>That is one of the paths Ze is built for. Ze aims for an easy migration from ExaBGP rather than perfect compatibility.</p>
      <p>There is a config converter (<code>ze config migrate</code>) and a compatibility bridge that lets existing ExaBGP process scripts run with Ze as the engine while you port them over. The <a href="../usage/exabgp-migration/">ExaBGP migration usage example</a> walks through the conversion, the known differences, and when it is worth rewriting a plugin against the native Ze SDK.</p>
    </div>
  </details>

  <details class="faq-card cat-secure" id="license">
    <summary>What license is Ze under, and what does that mean for me?</summary>
    <div>
      <p>Ze is free software under the <a href="../license/">GNU Affero General Public License v3</a>. You can run it, read it, modify it, and redistribute it.</p>
      <p>The AGPL adds one obligation beyond the GPL: if you offer a modified Ze to others over a network, you have to make your modified source available to those users. Running an unmodified Ze to route your own traffic carries no such obligation.</p>
    </div>
  </details>

  <details class="faq-card cat-automate" id="contribute-cla">
    <summary>I want to contribute. What is the CLA about?</summary>
    <div>
      <p>Contributions require a signed-off commit (<code>git commit -s</code>), which certifies agreement to the <a href="../contribute/">Contributor License Agreement</a>. You keep your copyright.</p>
      <p>What you grant is a broad license that lets the maintainer relicense the project, for example to offer a commercial license alongside the AGPL if that ever helps Ze reach more people. Ze stays AGPLv3 for everyone regardless. The <a href="../contribute/guide/">contributor guide</a> covers the rest of the process.</p>
    </div>
  </details>

  <details class="faq-card cat-operate" id="daemon-appliance">
    <summary>The daemon or the appliance: which should I run?</summary>
    <div>
      <p>The same binary and the same config drive both. Run it as a <strong>daemon</strong> when you are fitting Ze into infrastructure you already operate under systemd or another process manager.</p>
      <p>Build it as an <strong>appliance</strong> when you want a purpose-built box: a read-only root filesystem, no shell, no package manager, and automatic supervision, produced with gokrazy. Start with the daemon if you are unsure.</p>
    </div>
  </details>

  <details class="faq-card cat-observe" id="ai-mcp">
    <summary>Does the AI and MCP support mean Ze needs an LLM to run?</summary>
    <div>
      <p>No. Ze runs entirely on its own. The MCP server is an optional surface derived from the same schema and command catalogue. It lets an AI assistant ask what the running daemon, including compiled-in plugins, can do and then operate it.</p>
    </div>
  </details>

  <details class="faq-card cat-routing" id="config-stability">
    <summary>Will my configuration keep working as Ze changes?</summary>
    <div>
      <p>Until the first release, treat the configuration syntax as not yet frozen. Breaking changes are called out in the <a href="../changes/">changes log</a>, and the policy is no silent breakage: a change that affects your config should come with either an automatic migration or a clear error.</p>
      <p>Stabilising the syntax so it stays stable is an explicit milestone on the <a href="../roadmap/">roadmap</a>.</p>
    </div>
  </details>

  <details class="faq-card cat-services" id="funding">
    <summary>Who builds Ze, and how is it funded?</summary>
    <div>
      <p>Ze is developed by Thomas Mangin, with his time supported by <a href="https://exa.net.uk">Exa Networks</a>, the ISP that has backed this work since 2009 when it began with ExaBGP.</p>
      <p>There is no subscription, no paid support tier, and no separate commercial entity today. The <a href="../contribute/">contribute page</a> has the detail.</p>
    </div>
  </details>

  <details class="faq-card cat-secure" id="help">
    <summary>How do I get help, or report a security problem?</summary>
    <div>
      <p>For questions and discussion, use <a href="https://discord.gg/T8s7CjPDne">Discord</a> or the <a href="https://github.com/ze-software/ze/issues">issue tracker</a>.</p>
      <p>For anything security-sensitive, such as a bug an unauthenticated peer could trigger, follow the <a href="../security/">security policy</a> and report it privately instead of opening a public issue.</p>
    </div>
  </details>
</div>
