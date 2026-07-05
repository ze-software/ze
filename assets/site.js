document.documentElement.classList.add("js");

document.addEventListener("DOMContentLoaded", function () {
    var reveals = Array.prototype.slice.call(document.querySelectorAll(".reveal"));
    if (reveals.length) {
        if (!("IntersectionObserver" in window)) {
            reveals.forEach(function (el) {
                el.classList.add("visible");
            });
        } else {
            var observer = new IntersectionObserver(
                function (entries) {
                    entries.forEach(function (entry) {
                        if (entry.isIntersecting) {
                            entry.target.classList.add("visible");
                            entry.target.classList.remove("reveal-pending");
                            observer.unobserve(entry.target);
                        }
                    });
                },
                { threshold: 0.01 },
            );
            reveals.forEach(function (el) {
                el.classList.add("reveal-pending");
                observer.observe(el);
            });
        }
    }

    var header = document.querySelector(".site-header");
    if (header) {
        var lastY = window.scrollY;
        var hideThreshold = header.offsetHeight;
        var suppressHide = false;
        var suppressTimer = null;

        window.addEventListener(
            "scroll",
            function () {
                var y = window.scrollY;
                if (!suppressHide && y > lastY && y > hideThreshold) {
                    header.classList.add("hidden");
                } else {
                    header.classList.remove("hidden");
                }
                lastY = y;
            },
            { passive: true },
        );
        header.addEventListener("focusin", function () {
            header.classList.remove("hidden");
        });
        document.addEventListener("mousemove", function (event) {
            if (event.clientY <= 12) {
                header.classList.remove("hidden");
            }
        });

        document.querySelectorAll('a[href*="#"]').forEach(function (link) {
            link.addEventListener("click", function () {
                suppressHide = true;
                header.classList.remove("hidden");
                clearTimeout(suppressTimer);
                suppressTimer = setTimeout(function () {
                    suppressHide = false;
                }, 1000);
            });
        });
    }

    var menuToggle = document.querySelector(".nav-menu-toggle");
    var navLinks = document.getElementById("site-nav-links");
    if (menuToggle && navLinks) {
        function setMenu(open) {
            navLinks.classList.toggle("is-open", open);
            menuToggle.setAttribute("aria-expanded", open ? "true" : "false");
        }
        menuToggle.addEventListener("click", function () {
            setMenu(!navLinks.classList.contains("is-open"));
        });
        document.addEventListener("click", function (event) {
            if (!navLinks.classList.contains("is-open")) return;
            if (event.target.closest(".nav")) return;
            setMenu(false);
        });
        document.addEventListener("keydown", function (event) {
            if (event.key === "Escape") setMenu(false);
        });
    }

    function setDropdown(dropdown, open) {
        var trigger = dropdown.querySelector(".nav-dropdown-trigger");
        dropdown.classList.toggle("is-open", open);
        if (trigger) trigger.setAttribute("aria-expanded", open ? "true" : "false");
    }

    document.querySelectorAll(".nav-dropdown").forEach(function (dropdown) {
        var trigger = dropdown.querySelector(".nav-dropdown-trigger");
        if (!trigger) return;
        trigger.addEventListener("click", function (event) {
            event.preventDefault();
            var open = !dropdown.classList.contains("is-open");
            document.querySelectorAll(".nav-dropdown.is-open").forEach(function (other) {
                if (other !== dropdown) setDropdown(other, false);
            });
            setDropdown(dropdown, open);
        });
        dropdown.addEventListener("mouseenter", function () {
            trigger.setAttribute("aria-expanded", "true");
        });
        dropdown.addEventListener("mouseleave", function () {
            if (!dropdown.classList.contains("is-open")) {
                trigger.setAttribute("aria-expanded", "false");
            }
        });
        dropdown.addEventListener("focusin", function () {
            trigger.setAttribute("aria-expanded", "true");
        });
        dropdown.addEventListener("focusout", function (event) {
            if (!dropdown.contains(event.relatedTarget) && !dropdown.classList.contains("is-open")) {
                trigger.setAttribute("aria-expanded", "false");
            }
        });
    });

    document.addEventListener("click", function (event) {
        if (event.target.closest(".nav-dropdown")) return;
        document.querySelectorAll(".nav-dropdown.is-open").forEach(function (dropdown) {
            setDropdown(dropdown, false);
        });
    });

    document.addEventListener("keydown", function (event) {
        if (event.key !== "Escape") return;
        document.querySelectorAll(".nav-dropdown.is-open").forEach(function (dropdown) {
            setDropdown(dropdown, false);
        });
        var active = document.activeElement;
        if (active && active.closest(".nav-dropdown")) active.blur();
    });

    var searchBadge = document.querySelector(".nav-badge-search");
    if (searchBadge) {
        var root = searchBadge.getAttribute("href").replace(/search\/$/, "");
        var records = null;
        var loading = false;
        var previousFocus = null;
        var inerted = [];
        var overlay = document.createElement("div");
        overlay.className = "search-overlay";
        overlay.hidden = true;
        overlay.innerHTML =
            '<div class="search-overlay-backdrop" data-close></div>' +
            '<div class="search-overlay-panel" role="dialog" aria-modal="true" ' +
            'aria-labelledby="search-overlay-title">' +
            '<div class="search-overlay-head">' +
            '<h2 id="search-overlay-title" class="search-overlay-title">Search Ze</h2>' +
            '<button type="button" class="search-overlay-close" data-close ' +
            'aria-label="Close search">×</button>' +
            "</div>" +
            '<input type="search" class="search-overlay-input" ' +
            'placeholder="Search docs, config, CLI, blog..." ' +
            'aria-label="Search the site" />' +
            '<p class="search-overlay-status" role="status" aria-live="polite"></p>' +
            '<ul class="search-overlay-results" aria-label="Search results"></ul>' +
            "</div>";
        document.body.appendChild(overlay);
        var input = overlay.querySelector(".search-overlay-input");
        var status = overlay.querySelector(".search-overlay-status");
        var list = overlay.querySelector(".search-overlay-results");
        var closeButton = overlay.querySelector(".search-overlay-close");
        var ENT = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" };

        function esc(s) {
            return String(s).replace(/[&<>"]/g, function (c) {
                return ENT[c];
            });
        }

        function snippet(text, tokens) {
            var low = text.toLowerCase();
            var at = -1;
            for (var i = 0; i < tokens.length; i++) {
                var p = low.indexOf(tokens[i]);
                if (p !== -1 && (at === -1 || p < at)) at = p;
            }
            if (at === -1) at = 0;
            var start = Math.max(0, at - 60);
            var frag = text.slice(start, start + 200);
            if (start > 0) frag = "..." + frag;
            if (start + 200 < text.length) frag = frag + "...";
            frag = esc(frag);
            tokens.forEach(function (t) {
                if (!t) return;
                var re = new RegExp(
                    "(" + t.replace(/[.*+?^${}()|[\]\\]/g, "\\$&") + ")",
                    "ig",
                );
                frag = frag.replace(re, "<mark>$1</mark>");
            });
            return frag;
        }

        function score(r, tokens) {
            var title = (r.titleLower || r.title.toLowerCase());
            var section = (r.sectionLower || (r.section || "").toLowerCase());
            var text = (r.textLower || r.text.toLowerCase());
            var total = 0;
            for (var i = 0; i < tokens.length; i++) {
                var t = tokens[i];
                var hit = 0;
                if (title.indexOf(t) !== -1) hit += 12;
                if (section.indexOf(t) !== -1) hit += 4;
                var n = text.split(t).length - 1;
                if (n > 0) hit += Math.min(n, 5);
                if (hit === 0) return 0;
                total += hit;
            }
            return total;
        }

        function run(q) {
            list.innerHTML = "";
            if (!records) {
                status.textContent = loading ? "Loading index..." : "";
                return;
            }
            var tokens = q.toLowerCase().split(/\s+/).filter(Boolean);
            if (!tokens.length) {
                status.textContent = "";
                return;
            }
            var scored = [];
            for (var i = 0; i < records.length; i++) {
                var s = score(records[i], tokens);
                if (s > 0) scored.push([s, records[i]]);
            }
            scored.sort(function (a, b) { return b[0] - a[0]; });
            status.textContent = scored.length
                ? scored.length + " result" + (scored.length === 1 ? "" : "s")
                : 'No results for "' + q + '".';
            scored.slice(0, 30).forEach(function (pair) {
                var r = pair[1];
                var li = document.createElement("li");
                li.className = "search-result";
                li.innerHTML =
                    '<a href="' + esc(root + (r.url || "")) +
                    '"><span class="search-result-title">' + esc(r.title) +
                    '</span> <span class="chip">' + esc(r.section) + "</span></a>" +
                    '<p class="search-result-snippet">' + snippet(r.text, tokens) +
                    "</p>";
                list.appendChild(li);
            });
        }

        function load() {
            if (records || loading) return;
            loading = true;
            status.textContent = "Loading index...";
            fetch(root + "data/search-index.json")
                .then(function (r) { return r.json(); })
                .then(function (data) {
                    records = data.map(function (record) {
                        record.titleLower = record.title.toLowerCase();
                        record.sectionLower = (record.section || "").toLowerCase();
                        record.textLower = record.text.toLowerCase();
                        return record;
                    });
                    loading = false;
                    run(input.value);
                })
                .catch(function () {
                    loading = false;
                    status.textContent = "Could not load the search index.";
                });
        }

        function setBackgroundInert(on) {
            if (on) {
                inerted = [];
                Array.prototype.slice.call(document.body.children).forEach(function (el) {
                    if (el === overlay || el.tagName === "SCRIPT") return;
                    inerted.push({ el: el, aria: el.getAttribute("aria-hidden"), inert: el.inert });
                    el.setAttribute("aria-hidden", "true");
                    if ("inert" in el) el.inert = true;
                });
                return;
            }
            inerted.forEach(function (item) {
                if (item.aria === null) item.el.removeAttribute("aria-hidden");
                else item.el.setAttribute("aria-hidden", item.aria);
                if ("inert" in item.el) item.el.inert = item.inert;
            });
            inerted = [];
        }

        function focusable() {
            return Array.prototype.slice.call(
                overlay.querySelectorAll('a[href], button:not([disabled]), input:not([disabled])'),
            ).filter(function (el) {
                return el.offsetWidth || el.offsetHeight || el === document.activeElement;
            });
        }

        function open() {
            if (!overlay.hidden) return;
            previousFocus = document.activeElement;
            overlay.hidden = false;
            searchBadge.setAttribute("aria-expanded", "true");
            document.body.classList.add("search-open");
            setBackgroundInert(true);
            load();
            input.focus();
            input.select();
        }

        function close() {
            if (overlay.hidden) return;
            overlay.hidden = true;
            searchBadge.setAttribute("aria-expanded", "false");
            document.body.classList.remove("search-open");
            setBackgroundInert(false);
            if (previousFocus && typeof previousFocus.focus === "function") {
                previousFocus.focus();
            }
        }

        var debounceTimer;
        input.addEventListener("input", function () {
            clearTimeout(debounceTimer);
            debounceTimer = setTimeout(function () { run(input.value); }, 120);
        });
        overlay.addEventListener("click", function (event) {
            if (event.target.hasAttribute("data-close")) close();
        });
        overlay.addEventListener("keydown", function (event) {
            if (event.key === "Escape") {
                event.preventDefault();
                close();
                return;
            }
            if (event.key !== "Tab") return;
            var nodes = focusable();
            if (!nodes.length) return;
            var first = nodes[0];
            var last = nodes[nodes.length - 1];
            if (event.shiftKey && document.activeElement === first) {
                event.preventDefault();
                last.focus();
            } else if (!event.shiftKey && document.activeElement === last) {
                event.preventDefault();
                first.focus();
            }
        });
        searchBadge.addEventListener("click", function (event) {
            event.preventDefault();
            open();
        });
        closeButton.addEventListener("click", close);
        document.addEventListener("keydown", function (event) {
            if (event.key === "Escape" && !overlay.hidden) {
                event.preventDefault();
                close();
                return;
            }
            if ((event.key === "k" || event.key === "K") && (event.metaKey || event.ctrlKey)) {
                event.preventDefault();
                open();
                return;
            }
            if (event.key === "/" && overlay.hidden) {
                var el = document.activeElement;
                var tag = el && el.tagName;
                if (tag !== "INPUT" && tag !== "TEXTAREA" && !(el && el.isContentEditable)) {
                    event.preventDefault();
                    open();
                }
            }
        });
    }

    function initSourceLinks() {
        var sources = [
            {
                match: /^(internal|pkg|cmd|docs|schema|test|plan|mk|scripts)\//,
                base: "https://codeberg.org/thomas-mangin/ze/src/branch/main/",
                forge: "forgejo",
            },
            {
                match: /^(interface-definitions|include|data|python|smoketest|op-mode-definitions)\//,
                base: "https://github.com/vyos/vyos-1x/blob/current/",
                forge: "github",
            },
            {
                match: /^src\/conf_mode\//,
                base: "https://github.com/vyos/vyos-1x/blob/current/",
                forge: "github",
            },
            {
                match: /^Makefile$/,
                base: "https://github.com/vyos/vyos-1x/blob/current/",
                forge: "github",
            },
            {
                match: /^(src\/org\/freertr|cfg|misc)\//,
                base: "https://codeberg.org/mc36/freeRtr/src/branch/master/",
                forge: "forgejo",
            },
        ];

        function lineAnchor(forge, start, end) {
            if (!start) return "";
            if (forge === "github") return "#L" + start + (end ? "-L" + end : "");
            return "#L" + start + (end ? "-L" + end : "");
        }

        function sourceFor(text) {
            var match = text.trim().match(/^([^\s:]+)(?::(\d+)(?:-(\d+))?(?:,\d+(?:-\d+)?)*)?$/);
            if (!match) return "";
            var path = match[1];
            var javaDirs = {
                cfg: "src/org/freertr/cfg/",
                rtr: "src/org/freertr/rtr/",
                tab: "src/org/freertr/tab/",
                ip: "src/org/freertr/ip/",
                serv: "src/org/freertr/serv/",
                clnt: "src/org/freertr/clnt/",
                user: "src/org/freertr/user/",
                ifc: "src/org/freertr/ifc/",
                prt: "src/org/freertr/prt/",
            };
            Object.keys(javaDirs).some(function (prefix) {
                if (path.indexOf("/") !== -1 || !new RegExp("^" + prefix + "[A-Za-z0-9_]*\\.java$").test(path)) {
                    return false;
                }
                path = javaDirs[prefix] + path;
                return true;
            });
            for (var i = 0; i < sources.length; i++) {
                if (!sources[i].match.test(path)) continue;
                return sources[i].base + path + lineAnchor(sources[i].forge, match[2], match[3]);
            }
            return "";
        }

        Array.prototype.slice.call(document.querySelectorAll(".md-content code")).forEach(function (code) {
            if (code.closest("a")) return;
            var href = sourceFor(code.textContent);
            if (!href) return;
            var link = document.createElement("a");
            link.className = "source-link";
            link.href = href;
            link.target = "_blank";
            link.rel = "noopener";
            link.setAttribute("aria-label", "Open source evidence " + code.textContent.trim());
            code.parentNode.insertBefore(link, code);
            link.appendChild(code);
        });
    }


    function initFeatureTooltips() {
        var productNames = [
            "Ze",
            "VyOS",
            "freeRtr",
            "BIRD 3",
            "BIRD 2",
            "FRR",
            "OpenBGPd",
            "GoBGP",
            "bio-rd",
            "ExaBGP",
            "RustyBGP",
            "rustbgpd",
        ];
        var acronymGlossary = [
            ["AFI/SAFI", "AFI/SAFI (Address Family Identifier / Subsequent Address Family Identifier)"],
            ["ASN", "ASN (Autonomous System Number)"],
            ["BGP", "BGP (Border Gateway Protocol)"],
            ["BGP-LS", "BGP-LS (BGP Link-State)"],
            ["BFD", "BFD (Bidirectional Forwarding Detection)"],
            ["BMP", "BMP (BGP Monitoring Protocol)"],
            ["CoPP", "CoPP (Control Plane Policing)"],
            ["CLI", "CLI (Command Line Interface)"],
            ["DNS", "DNS (Domain Name System)"],
            ["DHCP", "DHCP (Dynamic Host Configuration Protocol)"],
            ["ECMP", "ECMP (Equal-Cost Multipath)"],
            ["EVPN", "EVPN (Ethernet VPN)"],
            ["FIB", "FIB (Forwarding Information Base)"],
            ["gNMI", "gNMI (gRPC Network Management Interface)"],
            ["gRPC", "gRPC (remote procedure call API)"],
            ["GR", "GR (Graceful Restart)"],
            ["GRE", "GRE (Generic Routing Encapsulation)"],
            ["GRETAP", "GRETAP (GRE Ethernet tap tunnel)"],
            ["GTSM", "GTSM (Generalized TTL Security Mechanism)"],
            ["HTMX", "HTMX (HTML-over-the-wire frontend library)"],
            ["HTTP", "HTTP (Hypertext Transfer Protocol)"],
            ["IGMP", "IGMP (Internet Group Management Protocol)"],
            ["IGP", "IGP (Interior Gateway Protocol)"],
            ["IPFIX", "IPFIX (IP Flow Information Export)"],
            ["IPIP", "IPIP (IP-in-IP tunnel)"],
            ["IPsec", "IPsec (IP Security)"],
            ["IS-IS", "IS-IS (Intermediate System to Intermediate System routing)"],
            ["JSON", "JSON (JavaScript Object Notation)"],
            ["L2TP", "L2TP (Layer 2 Tunneling Protocol)"],
            ["LAG", "LAG (Link Aggregation Group)"],
            ["LDP", "LDP (Label Distribution Protocol)"],
            ["LLDP", "LLDP (Link Layer Discovery Protocol)"],
            ["LLGR", "LLGR (Long-Lived Graceful Restart)"],
            ["LOCAL_PREF", "LOCAL_PREF (BGP local preference)"],
            ["MACsec", "MACsec (IEEE 802.1AE link encryption)"],
            ["MCP", "MCP (Model Context Protocol)"],
            ["MED", "MED (BGP Multi-Exit Discriminator)"],
            ["MPLS", "MPLS (Multiprotocol Label Switching)"],
            ["MRT", "MRT (Multi-threaded Routing Toolkit dump format)"],
            ["MSDP", "MSDP (Multicast Source Discovery Protocol)"],
            ["MTU", "MTU (Maximum Transmission Unit)"],
            ["MUP", "MUP (Mobile User Plane)"],
            ["MVPN", "MVPN (Multicast VPN)"],
            ["NAT", "NAT (Network Address Translation)"],
            ["NETCONF", "NETCONF (Network Configuration Protocol)"],
            ["NPTv6", "NPTv6 (IPv6 Network Prefix Translation)"],
            ["OSPF", "OSPF (Open Shortest Path First)"],
            ["PBR", "PBR (Policy-Based Routing)"],
            ["PIM", "PIM (Protocol Independent Multicast)"],
            ["PKI", "PKI (Public Key Infrastructure)"],
            ["PPP", "PPP (Point-to-Point Protocol)"],
            ["PPPoE", "PPPoE (PPP over Ethernet)"],
            ["PTP", "PTP (Precision Time Protocol)"],
            ["QoS", "QoS (Quality of Service)"],
            ["QinQ", "QinQ (stacked VLAN tagging)"],
            ["REST", "REST (HTTP resource API style)"],
            ["RFC", "RFC (IETF Request for Comments)"],
            ["RIB", "RIB (Routing Information Base)"],
            ["RIPng", "RIPng (Routing Information Protocol next generation)"],
            ["RPKI", "RPKI (Resource Public Key Infrastructure)"],
            ["RSVP-TE", "RSVP-TE (Resource Reservation Protocol Traffic Engineering)"],
            ["RTC", "RTC (Route Target Constraint)"],
            ["RTR", "RTR (RPKI-to-Router protocol)"],
            ["SDK", "SDK (Software Development Kit)"],
            ["SNMP", "SNMP (Simple Network Management Protocol)"],
            ["SRv6", "SRv6 (Segment Routing over IPv6)"],
            ["SSH", "SSH (Secure Shell)"],
            ["TACACS+", "TACACS+ (Terminal Access Controller Access-Control System Plus)"],
            ["TCP-AO", "TCP-AO (TCP Authentication Option)"],
            ["TCP MD5", "TCP MD5 (TCP MD5 Signature Option)"],
            ["TFTP", "TFTP (Trivial File Transfer Protocol)"],
            ["TTL", "TTL (Time To Live)"],
            ["TUN/TAP", "TUN/TAP (virtual network tunnel or tap devices)"],
            ["VLAN", "VLAN (Virtual LAN)"],
            ["VPLS", "VPLS (Virtual Private LAN Service)"],
            ["VPP", "VPP (Vector Packet Processing)"],
            ["VPN", "VPN (Virtual Private Network)"],
            ["VRF", "VRF (Virtual Routing and Forwarding)"],
            ["VXLAN", "VXLAN (Virtual Extensible LAN)"],
            ["WWAN", "WWAN (Wireless Wide Area Network)"],
            ["XDP", "XDP (eXpress Data Path)"],
            ["YANG", "YANG (data modeling language for network config)"],
        ];
        var featureGlossary = {
            "language": "Implementation language or runtime used by the project.",
            "license": "Project license for the compared codebase.",
            "primary interface": "Main operator or automation surface, such as CLI, HTTP API, gRPC, or SSH.",
            "first release": "Approximate first public release year used for context, not a quality score.",
            "multithreaded": "Whether the implementation can run useful routing work across multiple threads or workers.",
            "multithread model": "How concurrency is structured, such as goroutines, worker threads, processes, or per-peer tasks.",
            "plugin architecture": "Whether the project has an extension mechanism for loading or registering features outside the core.",
            "yang-modeled config": "Whether configuration is modeled with YANG or a similar schema-backed network config model.",
            "afi/safi": "BGP address family coverage. AFI selects the network family and SAFI selects the service type carried by BGP.",
            "route redistribution / protocol import-export": "Moving routes learned or produced by one protocol into another protocol, for example connected or static routes into BGP or IS-IS.",
            "is-is route leaking": "Controlled movement of routes between IS-IS Level 1 and Level 2 areas so reachability crosses the IS-IS hierarchy.",
            "cross-vrf route leaking": "Controlled movement of routes between separate VRF routing tables.",
            "rib / fib programming": "Ability to install routes into an internal routing table and push forwarding entries into the kernel or dataplane.",
            "vrf / routing instances": "Separate routing tables and interface bindings, usually used for tenant or service separation.",
            "policy-based routing": "Forwarding packets according to policy rules, such as source, mark, interface, or application criteria, not only destination prefix.",
            "vpp interface backend or vpp interface surface": "Whether the project can configure or target VPP-backed interfaces or exposes VPP-specific interface configuration.",
            "interface management model": "How the project represents and applies interface configuration across Linux, VPP, or other backends.",
            "native/p4/xdp dataplane helpers": "Project-owned support for programmable or accelerated dataplanes such as P4 or XDP, beyond normal Linux networking.",
            "generated command reference/cache": "Machine-generated command documentation or command metadata used by the CLI or docs.",
            "generated registries/schema artifacts": "Generated files built from source declarations, such as plugin registries, schema imports, command caches, or feature catalogs. This row asks whether the project has machine-built wiring for features and configuration rather than only hand-maintained lists.",
            "generated composition roots": "Generated startup wiring that imports or registers every component or plugin so the runtime discovers all shipped features.",
            "startup/command registration ownership": "Whether each feature owns its startup hooks and commands, so removing that feature also removes its CLI, config, and runtime wiring.",
            "schema validation": "Validation that config or API input follows the declared schema before it is applied.",
            "machine-readable config schema": "Whether automation can read a formal schema for configuration fields, types, and constraints.",
            "external plugin sdk/api": "Documented interfaces for external code to extend the product without changing the core source tree.",
            "internal and external plugin process model": "Whether extensions run in-process, as child processes, or both, and how they communicate with the core.",
            "external process integration": "A mechanism for external helper processes to receive events, provide commands, or extend runtime behavior.",
            "external process protocol": "A documented wire protocol used by out-of-process plugins or integrations.",
            "protobuf/grpc api boundary": "A typed API boundary defined with protobuf and exposed through gRPC.",
            "source-owned protocol implementations": "Protocol engines implemented in the project's own source tree rather than only configured through an external daemon.",
            "allocation/zero-copy tuning patterns": "Code patterns that reduce allocations and avoid unnecessary copies on hot paths.",
            "test seams and injectable components": "Interfaces or constructors that let tests replace real dependencies with controlled implementations.",
            "config verify/apply/rollback model": "Configuration workflow for checking a candidate config, applying it, and reverting safely when needed.",
            "candidate/draft config model": "A separate not-yet-active configuration tree that can be edited and checked before commit.",
            "commit confirm": "A safety workflow that automatically rolls back a config change unless the operator confirms it.",
            "control-plane hardening/sysctls": "Settings that protect the control plane or tune kernel network behavior, often through sysctl values.",
            "named copp feature": "A named Control Plane Policing feature that rate-limits or filters traffic destined to the router itself.",
            "mcp / ai integration": "Integration with the Model Context Protocol or AI assistant tooling.",
            "compile-time feature selection": "Build tags, package selection, or build profiles that choose which features are compiled into an image or binary.",
            "non-systemd appliance mode": "A runtime mode for appliance images that does not rely on systemd as the init or service manager.",
            "seed/bootstrap config database": "Initial configuration storage used to bring up a fresh install or appliance image.",
            "doctor/check framework": "Built-in checks that diagnose host readiness, runtime problems, or configuration issues.",
            "symptom-based diagnostics": "Diagnostics organized around observed symptoms, with explanations and remediation hints.",
            "verify/release evidence gates": "Automated checks used to prove a build, release, or documentation set is coherent before publishing.",
            "mutation testing": "Tests that deliberately alter code or logic to confirm the test suite catches meaningful regressions.",
            "runtime route injection": "Ability to add or withdraw routes at runtime without editing static configuration and restarting.",
            "hot reconfiguration (no restart)": "Changing runtime configuration without restarting the daemon or losing existing sessions.",
            "plugin-based policy": "Route policy implemented by registered plugins rather than only a built-in filter language.",
            "external process policy": "Route policy implemented through helper processes outside the daemon.",
            "custom filter language": "A project-specific policy language for matching and transforming routes.",
            "named policy definitions": "Reusable named policy objects that can be attached to peers, protocols, or import and export chains.",
            "route server and route reflector": "BGP roles that redistribute routes between clients without acting like a normal transit router.",
            "route server mode": "BGP route-server behavior for Internet exchange style peering, usually preserving client next-hop and policy semantics.",
            "dynamic neighbors": "Ability to accept or instantiate peers from ranges, listen sockets, or discovered addresses rather than only static peer entries.",
            "looking glass": "Operator or public route visibility interface for inspecting peers, prefixes, paths, and routing decisions.",
            "recursive next-hop": "Resolving a BGP next-hop through another route before programming forwarding.",
            "multipath/ecmp": "Installing multiple equal-cost paths for the same destination so traffic can be load-shared.",
        };

        function cleanLabel(text) {
            return text.replace(/\s+/g, " ").replace(/`/g, "").trim();
        }

        function productHeaderCount(table) {
            if (!table.rows.length) return 0;
            var header = table.tHead && table.tHead.rows.length ? table.tHead.rows[0] : table.rows[0];
            return Array.prototype.slice.call(header.cells).filter(function (cell) {
                var label = cleanLabel(cell.textContent);
                return productNames.indexOf(label) !== -1;
            }).length;
        }

        function escapeRegExp(text) {
            return text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
        }

        function expandedLabel(label) {
            var expansions = [];
            acronymGlossary.slice().sort(function (a, b) { return b[0].length - a[0].length; }).forEach(function (entry) {
                var pattern = new RegExp("(^|[^A-Za-z0-9+/-])" + escapeRegExp(entry[0]) + "(?=$|[^A-Za-z0-9+/-])");
                if (pattern.test(label)) expansions.push(entry[1]);
            });
            if (!expansions.length) return label;
            return label + " (" + expansions.join("; ") + ")";
        }

        function sectionFor(node) {
            var current = node;
            while (current) {
                current = current.previousElementSibling;
                if (current && current.tagName === "H2") return cleanLabel(current.textContent);
            }
            return "";
        }

        function descriptionFor(label, section) {
            var key = cleanLabel(label).toLowerCase();
            if (featureGlossary[key]) return featureGlossary[key];
            var description = "Compares product support for " + expandedLabel(label) + ".";
            description += " Read across the product columns. Yes or Present means supporting evidence was found, Partial means incomplete or delegated support, Unclear means the evidence was not strong enough, and No or Not found means the inspected sources did not show support.";
            if (section) description += " Section: " + section + ".";
            return description;
        }

        var targets = [];
        Array.prototype.slice.call(document.querySelectorAll(".md-content table")).forEach(function (table) {
            if (productHeaderCount(table) < 2) return;
            var section = sectionFor(table);
            Array.prototype.slice.call(table.querySelectorAll("tbody tr")).forEach(function (row) {
                var cell = row.cells[0];
                if (!cell || cell.querySelector(".feature-help")) return;
                var label = cleanLabel(cell.textContent);
                if (!label || label === "---") return;
                var help = document.createElement("span");
                var description = descriptionFor(label, section);
                help.className = "feature-help";
                help.tabIndex = 0;
                help.textContent = label;
                help.title = description;
                help.setAttribute("aria-label", label + ": " + description);
                help.setAttribute("data-feature-help", description);
                cell.textContent = "";
                cell.appendChild(help);
                targets.push(help);
            });
        });

        if (!targets.length) return;

        var popover = document.createElement("div");
        popover.className = "feature-tooltip-popover";
        popover.id = "feature-tooltip-popover";
        popover.setAttribute("role", "tooltip");
        popover.hidden = true;
        document.body.appendChild(popover);

        function position(target) {
            var rect = target.getBoundingClientRect();
            var tip = popover.getBoundingClientRect();
            var margin = 12;
            var left = Math.max(margin, Math.min(rect.left, window.innerWidth - tip.width - margin));
            var top = rect.bottom + 10;
            if (top + tip.height > window.innerHeight - margin) {
                top = Math.max(margin, rect.top - tip.height - 10);
            }
            popover.style.left = left + "px";
            popover.style.top = top + "px";
        }

        function show(target) {
            popover.textContent = target.getAttribute("data-feature-help");
            popover.hidden = false;
            popover.setAttribute("data-visible", "true");
            target.setAttribute("aria-describedby", popover.id);
            position(target);
        }

        function hide(target) {
            popover.removeAttribute("data-visible");
            popover.hidden = true;
            if (target) target.removeAttribute("aria-describedby");
        }

        targets.forEach(function (target) {
            target.addEventListener("mouseenter", function () { show(target); });
            target.addEventListener("mouseleave", function () { hide(target); });
            target.addEventListener("focus", function () { show(target); });
            target.addEventListener("blur", function () { hide(target); });
        });
        window.addEventListener("scroll", function () {
            var active = document.activeElement;
            if (active && active.classList && active.classList.contains("feature-help") && !popover.hidden) {
                position(active);
            }
        }, true);
        window.addEventListener("resize", function () {
            var active = document.activeElement;
            if (active && active.classList && active.classList.contains("feature-help") && !popover.hidden) {
                position(active);
            }
        });
    }
    function initComparisonFilters() {
        var productMatchers = [
            ["Ze", /^ze(?: evidence)?$/],
            ["VyOS", /^vyos(?: evidence)?$/],
            ["freeRtr", /^freertr(?: evidence)?$/],
            ["BIRD 3", /^bird 3$/],
            ["BIRD 2", /^bird 2$/],
            ["FRR", /^frr$/],
            ["OpenBGPd", /^openbgpd$/],
            ["GoBGP", /^gobgp$/],
            ["bio-rd", /^bio-rd$/],
            ["ExaBGP", /^exabgp$/],
            ["RustyBGP", /^rustybgp$/],
            ["rustbgpd", /^rustbgpd$/],
        ];

        function productLabel(text) {
            var normalized = text.replace(/\s+/g, " ").trim().toLowerCase();
            for (var i = 0; i < productMatchers.length; i++) {
                if (productMatchers[i][1].test(normalized)) return productMatchers[i][0];
            }
            return "";
        }

        function initColumnControls(tool, content, status) {
            var tables = Array.prototype.slice.call(content.querySelectorAll("table"));
            var columns = {};
            var order = [];

            tables.forEach(function (table) {
                if (!table.rows.length) return;
                var header = table.tHead && table.tHead.rows.length ? table.tHead.rows[0] : table.rows[0];
                Array.prototype.slice.call(header.cells).forEach(function (cell, index) {
                    var label = productLabel(cell.textContent);
                    if (!label) return;
                    if (!columns[label]) {
                        columns[label] = [];
                        order.push(label);
                    }
                    columns[label].push({ table: table, index: index });
                });
            });

            if (order.length < 3) return;

            var fieldset = document.createElement("fieldset");
            fieldset.className = "compare-columns";
            fieldset.setAttribute("data-compare-columns", "");
            fieldset.innerHTML = '<legend>Show products</legend>';
            var inputs = [];

            function setColumn(entry, visible) {
                Array.prototype.slice.call(entry.table.rows).forEach(function (row) {
                    var cell = row.cells[entry.index];
                    if (!cell) return;
                    cell.hidden = !visible;
                    cell.classList.toggle("compare-column-hidden", !visible);
                });
            }

            function applyColumns() {
                order.forEach(function (label) {
                    var input = fieldset.querySelector('input[value="' + label + '"]');
                    var visible = !input || input.checked;
                    columns[label].forEach(function (entry) {
                        setColumn(entry, visible);
                    });
                });
            }

            order.forEach(function (label) {
                var wrapper = document.createElement("label");
                wrapper.className = "compare-column-toggle";
                var input = document.createElement("input");
                input.type = "checkbox";
                input.value = label;
                input.checked = true;
                input.addEventListener("change", function () {
                    if (!inputs.some(function (node) { return node.checked; })) {
                        input.checked = true;
                    }
                    applyColumns();
                });
                inputs.push(input);
                wrapper.appendChild(input);
                wrapper.appendChild(document.createTextNode(label));
                fieldset.appendChild(wrapper);
            });

            tool.insertBefore(fieldset, status);
            applyColumns();
        }

        var tools = Array.prototype.slice.call(document.querySelectorAll("[data-compare-filter]"));
        tools.forEach(function (tool) {
            var input = tool.querySelector("[data-compare-search]");
            var select = tool.querySelector("[data-compare-section]");
            var status = tool.querySelector("[data-compare-status]");
            var content = tool.closest(".md-content");
            if (!input || !select || !status || !content) return;
            initColumnControls(tool, content, status);

            var headings = Array.prototype.slice.call(content.querySelectorAll("h2"));
            var groups = headings.map(function (heading, index) {
                var nodes = [];
                var node = heading.nextElementSibling;
                while (node && node.tagName !== "H2") {
                    if (node !== tool) nodes.push(node);
                    node = node.nextElementSibling;
                }
                var rows = [];
                nodes.forEach(function (groupNode) {
                    Array.prototype.slice.call(groupNode.querySelectorAll("tbody tr")).forEach(function (row) {
                        row.compareText = row.textContent.toLowerCase();
                        rows.push(row);
                    });
                });
                var option = document.createElement("option");
                option.value = String(index);
                option.textContent = heading.textContent.trim();
                select.appendChild(option);
                return {
                    heading: heading,
                    nodes: nodes,
                    rows: rows,
                    value: option.value,
                    text: (heading.textContent + " " + nodes.map(function (n) {
                        return n.textContent;
                    }).join(" ")).toLowerCase(),
                };
            });

            function setHidden(node, hidden) {
                node.classList.toggle("compare-hidden", hidden);
            }

            function applyFilter() {
                var query = input.value.trim().toLowerCase();
                var wanted = select.value;
                var visibleGroups = 0;
                var visibleRows = 0;
                var totalRows = 0;

                groups.forEach(function (group) {
                    var sectionAllowed = !wanted || wanted === group.value;
                    var groupTextMatch = !query || group.text.indexOf(query) !== -1;
                    var groupRowsVisible = 0;

                    group.rows.forEach(function (row) {
                        totalRows += 1;
                        var rowMatch = sectionAllowed && (!query || row.compareText.indexOf(query) !== -1);
                        setHidden(row, !rowMatch);
                        if (rowMatch) {
                            visibleRows += 1;
                            groupRowsVisible += 1;
                        }
                    });

                    var showGroup = sectionAllowed && (!query || groupTextMatch || groupRowsVisible > 0);
                    setHidden(group.heading, !showGroup);
                    group.nodes.forEach(function (node) {
                        if (node.tagName === "TABLE") {
                            setHidden(node, !sectionAllowed || (query && group.rows.length && groupRowsVisible === 0));
                        } else {
                            setHidden(node, !showGroup);
                        }
                    });
                    if (showGroup) visibleGroups += 1;
                });

                if (!groups.length) {
                    status.textContent = "No sections found.";
                } else if (!query && !wanted) {
                    status.textContent = totalRows + " table rows across " + groups.length + " sections.";
                } else {
                    status.textContent = visibleRows + " matching table rows in " + visibleGroups + " sections.";
                }
            }

            input.addEventListener("input", applyFilter);
            select.addEventListener("change", applyFilter);
            applyFilter();
        });
    }

    initFeatureTooltips();
    initComparisonFilters();
    initSourceLinks();
});
