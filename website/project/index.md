# Project overview

Follow Ze from its ExaBGP roots to a network operating system: what works today, what is changing, and what remains before the first release.

<div class="project-overview">
<section class="project-state" aria-labelledby="project-state-title">
    <div>
        <h2 id="project-state-title">Where Ze stands</h2>
        <p>Ze brings routing protocols, Linux networking and operator tools together around one configuration. It grew from ExaBGP's programmable BGP model, with a broader job: configure the network, inspect its state, and expose the same operations to people and automation.</p>
        <p>Ze is pre-release. You can explore it in a lab today; the available capabilities and their test evidence vary by feature.</p>
    </div>
    <a class="project-text-link" href="why-ze/">Why Ze, and where it fits <span aria-hidden="true">→</span></a>
</section>

<div class="project-progress">
    <section class="project-latest" aria-labelledby="project-latest-title">
        <p class="project-eyebrow">Recent work</p>
        <h2 id="project-latest-title">Latest development</h2>
        {project_latest}
        <p class="project-secondary-link"><a href="changes/">Browse all update reports</a></p>
    </section>
    <section class="project-release" aria-labelledby="project-release-title">
        <p class="project-eyebrow">Pre-release</p>
        <h2 id="project-release-title">Before the first release</h2>
        <p>The remaining work includes correctness fixes, validation of existing behavior, and release delivery. The roadmap separates these from later features and proposals.</p>
        <p>Work-item counts describe the backlog. They do not measure effort or production readiness.</p>
        <div class="project-links">
            <a href="roadmap/">Release roadmap <span aria-hidden="true">→</span></a>
            <a href="../quality/">Inspect the test evidence <span aria-hidden="true">→</span></a>
        </div>
    </section>
</div>

<section class="project-demo" aria-labelledby="project-demo-title">
    <div>
        <p class="project-eyebrow">A recorded session on a real lab router</p>
        <h2 id="project-demo-title">See Ze running</h2>
        <p>Watch an ExaBGP migration, inspect BGP peers, then configure, diagnose and repair services while the same router keeps running. The recordings include the commands and their output, with transcripts you can read at your own pace.</p>
        <div class="project-actions">
            <a class="button primary" href="../#product-terminal">Watch Ze in action <span aria-hidden="true">↗</span></a>
            <a class="project-text-link" href="../guides/quickstart/">Try the BGP quickstart <span aria-hidden="true">→</span></a>
        </div>
        <p class="project-secondary-link"><a href="../demos/terminal/#continuous-live-configuration-showcase">Open the continuous configuration recording and transcript</a></p>
    </div>
    <a class="project-demo-preview" href="../#product-terminal">
        <img src="../assets/product-dashboard.webp" alt="Ze's terminal dashboard showing BGP peers, session states and message counters" width="1384" height="811" loading="lazy" />
        <span>Preview from Ze's published terminal recording. Open the player to watch.</span>
    </a>
</section>

<section class="project-history" aria-labelledby="project-history-title">
    <p class="project-eyebrow">Recorded achievements</p>
    <h2 id="project-history-title">How we got here</h2>
    <p>These recent landmarks mark capabilities that arrived in Ze. The full timeline and project history explain how the system developed.</p>
    {project_landmarks}
    <div class="project-links">
        <a href="milestones/">Explore the milestone timeline <span aria-hidden="true">→</span></a>
        <a href="history/">From ExaBGP to Ze <span aria-hidden="true">→</span></a>
    </div>
</section>

<section class="project-development" aria-labelledby="project-development-title">
    <div>
        <h2 id="project-development-title">How Ze is built</h2>
        <p>AI coding agents write most of Ze's code within a framework of specifications agreed before implementation, tests written before code, independent reviews and commit checks. That process is iterative: code and the checks that exercise it improve as problems are found. Checking a first version against every requirement takes time and resources, so published code can still be imperfect.</p>
        <p>Most of this site's information is generated from the code. Its free text has more room for mistakes than protocol behavior defined by an RFC, so reports of unclear or incorrect information help improve it.</p>
        <a class="project-text-link" href="ai-development/">How Ze is developed with AI <span aria-hidden="true">→</span></a>
    </div>
    <nav class="project-reading" aria-label="Project reading and activity">
        <a href="../blog/"><strong>Engineering blog</strong><span>Longer accounts of the design and development.</span></a>
        <a href="../talks/"><strong>Talks</strong><span>Presentations about Ze and ExaBGP.</span></a>
        <a href="activity/"><strong>Development activity</strong><span>Repository history and the scope of the code.</span></a>
    </nav>
</section>

<section class="project-participation" aria-labelledby="project-participation-title">
    <div>
        <h2 id="project-participation-title">Get involved</h2>
        <p>Try Ze in a lab and report a failure with its configuration, trace or transcript. Documentation corrections, interoperability results, tests and focused code changes all help the project move forward.</p>
    </div>
    <a class="button" href="../contribute/">Contribute to Ze <span aria-hidden="true">→</span></a>
</section>
</div>
