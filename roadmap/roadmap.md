# Roadmap

Ze is pre-release. This page is about one thing: the work that stands between
today and a first release you can trust in production. It is a sequence, not a
calendar. Each step gates the next, and the honest answer to "when" is "when
the step before it is genuinely done".

<div class="roadmap-forward-note">
  <p>The <a href="../changes/">changes log</a> records what has already shipped week by week. This page looks forward.</p>
</div>

<div class="roadmap-step-list">
  <section class="roadmap-step">
    <h2>1. Close the open specs</h2>
    <p>Ze is built spec by spec: nothing gets written without a spec that says what it should do and how it will be tested. There is still a backlog of open specs, and the first job is to work through it so the feature set stops being a moving target. Until that is done, the surface keeps growing, and a growing surface is the wrong thing to try to stabilise.</p>
    <div class="roadmap-chip-row" aria-label="Step focus">
      <span class="roadmap-chip" data-tone="platform">Specs</span>
      <span class="roadmap-chip" data-tone="operate">Tests</span>
      <span class="roadmap-chip" data-tone="observe">Scope control</span>
    </div>
  </section>

  <section class="roadmap-step">
    <h2>2. Make it stable</h2>
    <p>Once the feature set stops moving, the effort turns to hardening what is there: chasing down races, tightening the tests, and closing the known trust-model gaps tracked in the <a href="../security/">security policy</a>. The routing core is already heavily tested, but "heavily tested" and "boring to run for months" are not the same thing, and the second one is the goal.</p>
    <div class="roadmap-chip-row" aria-label="Step focus">
      <span class="roadmap-chip" data-tone="secure">Security</span>
      <span class="roadmap-chip" data-tone="routing">Routing core</span>
      <span class="roadmap-chip" data-tone="operate">Reliability</span>
    </div>
  </section>

  <section class="roadmap-step">
    <h2>3. Prove it in production</h2>
    <p>Tests and lab interop take Ze a long way, but they are not a substitute for real traffic on real hardware over real time. This step is about operational mileage: running Ze in places where it has to keep working, watching what breaks, and fixing it. This is also where feedback from operators matters most, which is exactly why the project is open this early.</p>
    <div class="roadmap-chip-row" aria-label="Step focus">
      <span class="roadmap-chip" data-tone="services">Real traffic</span>
      <span class="roadmap-chip" data-tone="observe">Telemetry</span>
      <span class="roadmap-chip" data-tone="platform">Hardware</span>
    </div>
  </section>

  <section class="roadmap-step">
    <h2>4. Freeze the configuration syntax</h2>
    <p>Today the configuration syntax can still change, and the site says so. Before a release, the syntax has to settle so that a config you write keeps working. The policy is no silent breakage: any change that affects an existing config comes with either an automatic migration or a clear error. Getting the syntax to a point where it stops needing to change is the last big piece before release.</p>
    <div class="roadmap-chip-row" aria-label="Step focus">
      <span class="roadmap-chip" data-tone="automate">Migration</span>
      <span class="roadmap-chip" data-tone="operate">Validation</span>
      <span class="roadmap-chip" data-tone="secure">No silent breakage</span>
    </div>
  </section>

  <section class="roadmap-step">
    <h2>5. Release</h2>
    <p>The first release is what all of the above adds up to: a frozen configuration model, a stable feature set, and enough production mileage to stand behind it. Upgrade paths will be provided from that point on.</p>
    <div class="roadmap-chip-row" aria-label="Step focus">
      <span class="roadmap-chip" data-tone="platform">Frozen model</span>
      <span class="roadmap-chip" data-tone="observe">Production evidence</span>
      <span class="roadmap-chip" data-tone="automate">Upgrade paths</span>
    </div>
  </section>

  <section class="roadmap-step roadmap-step-muted">
    <h2>What is not the priority right now</h2>
    <p>New large features are not the focus on the way to the first release. Stability comes first. Known gaps against other daemons, such as BGP confederations, are listed honestly on the <a href="../compare/">comparison page</a>, and the <a href="../features/">features page</a> marks what is shipped versus still experimental. If something you need is missing or you want to help move a milestone along, say so on <a href="https://discord.gg/T8s7CjPDne">Discord</a> or the <a href="https://github.com/ze-software/ze/issues">issue tracker</a>.</p>
    <div class="roadmap-chip-row" aria-label="Step focus">
      <span class="roadmap-chip" data-tone="meta">Not now</span>
      <span class="roadmap-chip" data-tone="routing">Known gaps</span>
      <span class="roadmap-chip" data-tone="operate">Feedback</span>
    </div>
  </section>
</div>
