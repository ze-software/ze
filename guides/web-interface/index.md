# Web Interface

Ze provides an HTTPS web interface for browsing configuration, editing values, and running commands through a browser. The web UI uses the same YANG schemas, user database, and command grammar as the SSH CLI.
<!-- source: internal/component/web/handler.go -- URL routing, three-tier scheme -->
<!-- source: internal/component/web/auth.go -- authMiddleware, AuthMiddlewareWithAudit -->
<!-- source: cmd/ze/hub/service_web.go -- buildWebService -->
<!-- source: cmd/ze/hub/aaa_lifecycle.go -- liveAAABundleAuthenticator, liveAAABundleAuthenticator.Authenticate -->
<!-- source: internal/component/authz/auth.go -- LocalAuthenticator.Authenticate, authenticateUser -->

## Starting the Web Server

### Command Line

```bash
ze start --web 8443                              # Start daemon + web on port 8443
ze start --web 8443 --insecure-web               # No authentication (forces 127.0.0.1)
```

When no certificate is configured, ze generates an ECDSA P-256 self-signed certificate automatically. The certificate includes SANs for localhost, 127.0.0.1, ::1, and the listen address. `WebCertHosts` builds that list, and a listen address of `0.0.0.0` adds every non-loopback interface address of the machine.
<!-- source: cmd/ze/ze_core_start.go -- cmdStart, flagStartWeb, flagStartInsecureWeb -->

When no config file is named, the daemon reads the config name from the store. If the store's instance name cannot be read, `ze start` exits 1 with an error naming `ze doctor`, and a web server started with no config path stays off and prints `warning: web server disabled:` with the reason.
<!-- source: cmd/ze/hub/service_web.go -- startWebServer -->
<!-- source: internal/core/selfcert/selfcert.go -- GenerateWebCertWithAddr, GenerateWebCertWithNames, WebCertHosts -->

| Flag | Description |
|------|-------------|
| `--web <port>` | Start web interface on `0.0.0.0:<port>` (requires config) |
| `--web-only` | Start web UI only, no daemon (config editing only, default port 3443) |
| `--insecure-web` | Disable authentication (forces `127.0.0.1`, requires `--web` or `--web-only`) |

### Choosing an Interface

The default **YANG editor** uses Finder columns to browse the configuration and
edit its schema-backed fields. The **CLI** link opens `/cli`, where commands use
the shared configuration editor and the daemon's command dispatcher through the
browser, without an SSH connection.

**Workbench (experimental)** remains available as an unfinished alternative.
Its purpose-built pages use the same authentication and candidate configuration,
but some Workbench workflows are still incomplete. The controls at the top of
the page switch between the graphical editor, CLI and Workbench; an explicit
selection of Workbench remains selected for that browser.

The startup setting is `environment web ui-mode`, with values `finder` (the
default) and `workbench`. The equivalent environment key is `ze.web.ui-mode`:

```bash
env ze.web.ui-mode=workbench ze start --web 8443
```

<!-- source: internal/component/web/ui_mode.go -- parseUIMode, GetUIMode, ReadUIModeFromRequest -->
<!-- source: internal/component/web/component_breadcrumb.templ -- topbarActions -->

### Configuration

The web server listen address can also be set in the ze configuration file:

```
environment {
    web {
        enabled true;
        server main {
            ip 0.0.0.0;
            port 8443;
        }
    }
}
```
<!-- source: internal/component/web/yang/ze-web-conf.yang -- web container, server list -->

## Offline backup workspace

Choose **Services → Backup workspace**, or open `/backups/`, to edit a `.zefs`
backup independently of the running configuration. This page uses a full-text
configuration editor. It requires an account with configuration edit access,
because the artifact can contain credentials.
<!-- source: internal/component/web/register_backup_workspace.go -- init -->
<!-- source: internal/component/web/backup_workspace.go -- HandleBackupWorkspace, ServeHTTP -->
<!-- source: internal/component/web/page_backup_workspace.templ -- pageBackupWorkspace -->

To make a new backup of the system serving this page, choose **Create backup of
this system**. The browser downloads `system-backup.zefs`, containing the complete
store: configurations, history, saved drafts, credentials and other stored keys.
Creating the backup does not commit pending edits or change uploaded workspaces.
The system must have an open device store, and the complete snapshot must fit
the workspace's size and temporary-storage allowance. A refused creation returns
an explanation instead of a partial backup.
<!-- source: internal/component/web/backup_transfer.go -- downloadSystem -->
<!-- source: internal/component/config/storage/cli/system_backup.go -- BackupSystemTo -->

Before opening the new download in the Ze CLI on Unix, run
`chmod 600 system-backup.zefs`. Keep the backup private because it contains
credentials. You can upload it here to inspect or edit its configuration.

The server keeps its private backup working copies under the platform data
directory's `backups` folder: `/var/lib/ze/backups` on an installed Linux system
and `/perm/var/lib/ze/backups` on the appliance. To choose a different folder:

```text
environment {
    backup {
        directory "/srv/ze-backups";
    }
}
```

Use an absolute path the daemon user can create or write to, then commit the change.
Ze normalizes separators and `.` components to keep private roots stable and
refuses any `..` component; name the intended absolute folder directly.
The `ze.backup.directory` environment override takes priority over this leaf.
The commit creates a missing folder and checks write access as the running
daemon account. A successful commit selects it for new workspaces and downloads
without restarting Ze. A failed commit keeps the previous folder. Existing
workspaces and downloads keep their private files until they close or expire;
empty retired roots are then removed. Removing the leaf restores the platform
default unless the OS override is set. Ze preserves an existing parent's
permissions and unrelated files and refuses an unusable explicit location.
An unconfigured development
binary whose platform data directory cannot be resolved keeps the operating
system's temporary folder. Browser downloads go to the location chosen in the
browser's download settings.
<!-- source: internal/component/hub/yang/ze-hub-conf.yang -- environment, backup, directory -->
<!-- source: internal/component/config/artifact/directory.go -- configuredBackupParent, backupDirectory -->
<!-- source: internal/component/config/artifact/directory.go -- PrepareDirectory, Complete -->
<!-- source: internal/component/config/artifact/owner.go -- ensureRoot -->

The backup workspace also runs with `ze start --web-only`. It uses the same
artifact commands as the configured daemon; opening it starts no routing
protocol. Stop the web process only after downloading the changes you need.
<!-- source: cmd/ze/hub/service_web.go -- runWebOnly, webOnlyDispatcher -->

Full-store restore also uses the normal recovery lifecycle in web-only mode.
It waits for web requests and the original store owner to finish before
replacing the store. A cleanup failure prevents replacement and returns a
failure with the recovery instructions. Applying only the running configuration
requires a loaded daemon; web-only mode refuses that operation before staging
any configuration candidate.
<!-- source: cmd/ze/hub/web_only_owner.go -- bindWebOnlyRecovery, shutdownWebOnly, webOnlyExitStatus -->
<!-- source: cmd/ze/ze_core_start_recovery.go -- runOwnedDaemonStore -->
<!-- source: internal/component/config/storage/cli/data_rpc.go -- restoreDaemonConfig -->

1. Select a backup file and choose **Open backup**.
2. Choose its **Configuration**, then **Open configuration**. The editor starts
   with the committed configuration. Existing saved drafts and pending
   candidates remain in the artifact and are not resumed.
3. Edit the text and choose **Validate**. A refused edit keeps your text available
   for correction. Validation covers the checks available in this Ze process;
   it does not prove acceptance on the target host or running system.
4. Choose **Review changes**, then **Save into backup**. Changing the text after
   review requires another review. A form opened before another tab saved the
   same configuration must be reviewed against the newer saved configuration.
5. Choose **Download saved backup**. The download is a complete artifact with
   the edited configuration, its history and the original backup's other keys.
   Unsaved text is not included.
<!-- source: internal/component/web/backup_edit.go -- edit -->
<!-- source: internal/component/config/artifact/edit.go -- Edit, prepare, publish -->
<!-- source: internal/component/web/backup_transfer.go -- upload, download -->
<!-- source: internal/component/config/storage/artifact_workspace.go -- PublishArtifactConfig -->

6. Before opening the downloaded backup in the Ze CLI on Unix, restrict access to your account:

   ```sh
   chmod 600 updated-backup.zefs
   ```
<!-- source: internal/component/web/page_backup_workspace.templ -- pageBackupWorkspace -->
<!-- source: internal/component/config/storage/tree.go -- secureNode, openNode -->

7. Choose **Close workspace** when finished.
<!-- source: internal/component/web/backup_edit.go -- edit -->

Each workspace has its own URL and is available only to the account that
uploaded it. Uploaded copies are private temporary files; the original file on
your computer is unchanged. An artifact can be at most 64 MiB, and configuration
text can be at most 4 MiB. The server retains at most 32 workspaces, eight per
account, with a shared 256 MiB artifact allowance. Workspaces expire after one
hour without use and are removed when the daemon stops. Download the edited
backup before closing the workspace or stopping the daemon.

The same private artifact can be used from the command surface. The artifact
identity comes from upload completion and belongs to your authenticated account.
The browser's **Admin Commands** forms derive these commands and their fields
from the command model:

| Operation | Command |
|-----------|---------|
| Stored configurations | `show config artifact config list artifact <id>` |
| Masked committed source | `show config artifact source artifact <id> name <config>` |
| Resolved comparison | `show config artifact diff artifact <id> name <config> other <config>` |
| Compare a revision | `show config artifact diff artifact <id> name <config> revision <number>` |
| Stored history and draft state | `show config artifact history artifact <id> name <config>` |
| Set, activate or deactivate | `request config artifact <operation> artifact <id> name <config> text <transfer>` |
| Complete replacement | `request config artifact replace artifact <id> name <config> text <transfer>` |
| Publish a history revision | `request config artifact rollback artifact <id> name <config> revision <number>` |

For a statement or replacement, upload the text with `request config artifact
text begin`, the declared upload append operation and upload finish. Keep the
returned text-transfer identity until the operation ends, then abort it. A set
transfer contains one complete `set` statement; activation transfers similarly
contain one `activate` or `deactivate` statement. A replacement contains the
configuration. Adding `dry-run` returns a masked preview without changing the
retained artifact. These operations preserve saved drafts and unrelated keys.
<!-- source: internal/component/config/cli/artifact_config.go -- artifactConfigMutation, artifactReplace, artifactRollback -->
<!-- source: internal/plugins/config-cli/yang/ze-config-artifact-cmd.yang -- native-change, revision -->

To check a damaged backup in the browser, open **Check or repair a data file**
on `/backups/`. Select the file, or expand **Paste base64 data instead**, then
open the source. Both forms accept up to 64 MiB after decoding. The data need
not be a valid ZeFS container to upload.

Choose **Check source** to see the native integrity result. A failed check keeps
its finding report and the complete uploaded source available. **Create separate
salvage** produces an independent artifact; skipped keys or history entries stay
visible as warnings, and an unresolved failure remains a failure even when a
salvage can be downloaded. Inspect the report before using that download.
**Download original source** preserves the uploaded bytes, including after
repair. **Close source** removes only the uploaded copy. Sources expire after
two minutes without use, so download what you need before closing or leaving.
<!-- source: internal/component/web/backup_source.go -- uploadSource, source, downloadSource -->
<!-- source: internal/component/web/component_backup_source.templ -- componentBackupSourceUpload, componentBackupSource -->

For raw maintenance from a command surface, `show config artifact key list artifact <id>` lists keys.
`request config artifact key read artifact <id> key <key>` returns a private
download transfer; read its chunks and abort it afterward. To write a key, begin
a `request config artifact bytes begin name <label>` transfer, append and finish
its bytes, then use `request config artifact key write artifact <id> key <key>
transfer <source>`. Binary and empty values are retained without text conversion.
Explicit `request config artifact key delete artifact <id> key <key>` removes
only that key.

`show config artifact check transfer <source>` checks complete source bytes,
including a corrupt backup. `request config artifact repair transfer <source>`
returns a separate downloadable salvage. Keep the source until you have checked
the salvage: skipped or dropped keys produce warnings, and unresolved history
is a failed repair even though recovered keys remain available. A normal
workspace can be checked with `show config artifact check artifact <id>`.
<!-- source: internal/component/config/cli/artifact_raw.go -- artifactCheck, artifactRepair -->

To reopen saved offline CLI edits deliberately, use `request config artifact
session resume artifact <id> name <config>`. This resumes your account's saved
`local` editing origin and keeps other accounts' pending work separate. The
`session set`, `activate`, `deactivate` and `delete` operations take a text
transfer containing one matching editing statement. `session replace` and
`merge` take configuration text. `session save` retains your draft; `session
diff`, `changes` and `history` inspect it. `session commit` validates and
publishes your changes; `session discard` removes your pending intent. The
ordinary configuration page continues to open committed content.
<!-- source: internal/component/config/cli/artifact_session.go -- artifactSessionOperation -->

`request config artifact restore full artifact <id>` requests a complete device
store recovery. An accepted response confirms that independent recovery bytes
are retained; the running configuration remains in place through the response.
The daemon then restarts, and its outer owner imports the source only after a
successful shutdown and store close. Download the backup before the restart if
you also need to retain your own copy.
<!-- source: internal/component/config/cli/artifact_restore.go -- artifactRestoreFull -->
Saving and downloading also need temporary disk space for a complete working
copy. If private-file cleanup fails, new uploads, saves and downloads wait until that
cleanup succeeds; existing workspace inspection remains available.
<!-- source: internal/component/config/artifact/owner.go -- Owner, expire, Close -->
<!-- source: internal/component/config/artifact/transfer.go -- Begin, Append, Finish, Snapshot, Abort -->

## Authentication

The web interface uses the same user database as the SSH server. Users log in through a browser login page or authenticate API requests with HTTP Basic Auth.
<!-- source: internal/component/web/auth.go -- authMiddleware, loginHandler, parseBasicAuth -->

### Browser Sessions

1. Navigate to `https://<host>:8443/`. Unauthenticated requests receive a login page.
2. Enter username and password. On success, a `ze-session` cookie is set.
3. The cookie is `Secure`, `HttpOnly`, and `SameSite=Strict`.
4. Each user can have one active session. Logging in again invalidates the previous session.
5. A session lasts 24 hours, and ends earlier if the configuration stops declaring the user or changes that user's own password, profiles or SSH keys. Remove a user or change their password and reload, and their open browser tab is refused on its next request. A reload that changes only other users (adding a user, changing another user's password) leaves the session intact. No daemon restart is necessary.

A user authenticated by RADIUS or TACACS+ keeps their session: the local user list did not grant it and does not end it.

### JSON API

API clients that send `Accept: application/json` (or append `?format=json` to the URL) can authenticate with HTTP Basic Auth instead of session cookies. No session is created for Basic Auth requests.

```bash
curl -k -u admin:password https://localhost:8443/show/bgp/?format=json
```

### Security Headers

Every response carries `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self'`, and HSTS (`max-age=63072000; includeSubDomains`). An authenticated response adds `Cache-Control: no-store` and `X-Ze-Version`.

`X-Ze-Version` carries the release, the git commit, the Go version and the OS, which tells a client the exact build it speaks to. Write `environment { hide-version true; }` to keep it off every response of the web interface and the looking glass together. The default is false, so the banner stays until you hide it, and no other header changes. Ze sends no `Server` header at all.
<!-- source: internal/component/web/auth.go -- setSecurityHeaders, addSecurityHeaders -->
<!-- source: internal/core/version/version.go -- HTTPHeaderHidden -->

`script-src 'self'` refuses an inline script and refuses `Function()`. No page therefore carries an inline event handler, and no htmx attribute uses a bracketed trigger filter, because htmx compiles such a filter into source and calls `Function()` on it. A test refuses both in any `.templ` source.
<!-- source: internal/component/web/markup_contract_test.go -- TestNoTriggerFilterNeedsEval, TestSelfReplacingControlsCarryAStableID -->

### Secret Masking

No render path prints a value the schema marks `ze:sensitive` or `ze:bcrypt`. One
predicate, `config.LeafHoldsSecret`, answers whether a leaf holds a secret. The
display mask and the write guard both read it, so the two halves cannot drift
apart.

| Surface | Behavior |
|---------|----------|
| The config tree, the diff, the compare view and the download | A secret leaf that holds a value reads as the placeholder. An unset secret stays empty, so the field still reads as unconfigured |
| The commit diff | A rotated secret is named as a changed path. Neither the old value nor the new one is printed |
| The browser CLI | `show` masks the same leaves for authenticated users |
| The terminal's answer to `set` | The acknowledgement reads `set <leaf> /* SECRET-DATA */`, so the value never travels back in the response body |
| Commit, load and upload | A tree carrying the placeholder in a secret leaf is refused. Restore the real value from the edit-authorized raw download, or set it through `plaintext-<name>` or `ze passwd` |

<!-- source: internal/component/config/mask.go -- LeafHoldsSecret, MaskSecrets, ChangedSecretPaths, DisplayValueAtPath, RejectMaskedSecretLeaves -->
<!-- source: internal/component/web/cli_editor.go -- executeTerminalEditorCommand -->
<!-- source: internal/component/cli/model_commands_edit.go -- cmdSet -->

## Rendering

Every page, panel, fragment and out-of-band swap is written in a `.templ` source
and compiled to Go by [templ](https://templ.guide). No Go file in the package
builds markup: the exemption table is empty and a test holds it at zero. A
renamed view-model field is therefore a compile error, where `html/template`
rendered a blank panel and answered 200.

| Command | What it does |
|---------|--------------|
| Scoped native templ generation | Regenerates the selected `*_templ.go` from its `.templ` source; example below |
| `./le doc check templ-output` | Refuses stale generated output, orphaned `*_templ.go` files, and `.templ` sources outside the generator walk |

The native job runs the vendored generator. For example, after changing the
supplied-input controls:

```bash
./le job run label templ-gen command go run -mod=vendor github.com/a-h/templ/cmd/templ generate -keep-orphaned-files -path "$PWD/internal" -f internal/component/web/component_graphical_supplied.templ
```

Choose the edited `.templ` source with `-f` and retain the `internal/` root in
`-path`: that root is written into generated runtime filenames and must match
the checker. `./le repo update` has no templ stage. The complete workflow is in
[Testing](../../contribute/testing/index.md).
<!-- source: internal/le/doc/wiring/templ.go -- answerTemplOutput -->

The browser assets are vendored in `third_party/web/` and copied to each
consumer by the same `./le repo update` run. htmx 4.0.0 and its
`hx-sse.min.js` extension serve the web interface, the looking glass and the
chaos dashboard. `internal/le/web/assets.Write` derives each page's asset set
from its component graph, so a page loads only what it reaches. Each
third-party package travels with its licence file, which the interface and the
looking glass serve beside the asset.

<!-- source: internal/component/web/markup_check_test.go -- webMarkupExempt, TestNoGoFileBuildsMarkup -->
<!-- source: third_party/web/MANIFEST.md -- vendored versions, consumers, sync targets -->

## Navigation

### URL Scheme

URLs follow a verb-first three-tier pattern. A segment that is also a canonical
CLI verb (`show`, `monitor`, and `set`, `delete`, `commit` under `/config/`)
takes its spelling from `command.Verbs`, so the web URL and the CLI command
cannot drift apart. `config`, `admin`, `portal`, `login`, `assets` and the
editor words `edit`, `add`, `form`, `add-form`, `changes`, `rename`, `discard`
and `compare` are this router's own and the verb registry holds none of them.

<!-- source: internal/component/web/handler.go -- ParseURL, knownPrefixes, configVerbs -->
<!-- source: cmd/ze/hub/service_web.go -- buildWebService, canonical show routing -->
<!-- source: internal/component/web/handler_workbench.go -- HandleWorkbench, per-user editable fields -->
<!-- source: internal/component/command/verbs.go -- VerbShow, VerbMonitor, VerbSet, VerbDelete, VerbCommit -->
<!-- source: internal/component/web/handler_config_form.go -- HandleConfigDeleteWithAuthorizer -->

| Tier | URL Pattern | Method | Description |
|------|-------------|--------|-------------|
| View | `/show/<yang-path>` | GET | Config tree view |
| View | `/monitor/<yang-path>` | GET | View with auto-polling |
| Config | `/show/<path>` | GET | Editable config tree view using the authenticated user's pending changes |
| Config | `/config/set/<path>` | POST | Set a leaf value |
| Config | `/config/add/<path>` | POST | Create a list entry (with optional field values) |
| Config | `/config/add-form/<path>` | GET | Fetch add-entry overlay form |
| Config | `/config/rename/<path>` | POST | Rename a keyed list entry |
| Config | `/config/delete/<path>` | POST | Delete the node named by the `leaf` form field: a leaf, a leaf-list member, a container, a whole list, or one list entry (the delete button on a list row) |
| Config | `/config/commit/` | GET/POST | View diff and commit changes |
| Config | `/config/discard/` | POST | Discard pending changes |
| Config | `/config/changes` | GET | Commit bar state (pending change count) |
| Config | `/config/compare/` | GET | Compare pending vs committed |
| Admin | `/admin/<yang-path>` | GET/POST | Administrative commands |
| Auth | `/login` | POST | Login (no auth required) |
| Static | `/assets/` | GET | CSS, JS, images (no auth required) |

The root URL `/` redirects to `/show/`.

### Finder Navigation

The left panel uses a Finder-style column browser (similar to macOS Finder). It shows up to 3 columns, scrolling horizontally as you navigate deeper.
<!-- source: internal/component/web/fragment.go -- buildFinderColumns, buildColumnAt -->

**Named vs unnamed containers:** Named containers (lists with YANG keys, like `peer`, `group`) appear above unnamed containers (global settings like `local`, `timer`), separated by a horizontal rule. This makes keyed sections easy to find.

**Simple lists:** Lists without unique constraints show as a flat column of clickable entries with a `+ new` button.

### Context Heading

When inside a list entry, the detail panel shows a context heading at the top with the list name and entry key (e.g., `PEER london`). This provides immediate context without checking the breadcrumb.
<!-- source: internal/component/web/fragment.go -- buildContextHeading, ContextEntry -->

### List Table View

Lists that have YANG `unique` constraints (e.g., `peer` with `unique "remote/ip"`) display as an interactive table in the detail panel. The table shows the list key and all unique fields as columns.
<!-- source: internal/component/web/fragment.go -- buildListTable, collectUniqueFields -->
<!-- source: internal/component/web/component_list_table.templ -->

| Column | Behavior |
|--------|----------|
| Rename button | Opens a modal, validates the new key against the schema, and renames the entry without losing its subtree |
| Key column (e.g., name) | Clickable link, navigates into the entry's config subtree |
| Unique field columns (e.g., remote/ip) | Editable inline, saves on blur/Enter/auto-save (1s debounce) |
| Delete button | Removes the entry after confirmation |

The `+ new` button below the table opens a server-rendered form (via HTMX) with inputs for the entry name and all unique fields. Field values are validated against YANG types before the entry is created.
<!-- source: internal/component/web/handler_config_entry.go -- HandleConfigAddWithAuthorizer, HandleConfigAddForm -->
<!-- source: internal/component/web/component_add_form_overlay.templ -->

### Breadcrumb Navigation

Every page displays a breadcrumb trail from root to the current YANG path. Clicking any breadcrumb segment navigates to that level.
<!-- source: internal/component/web/handler_config_leaf.go -- buildBreadcrumbs -->

### Content Negotiation

The response format is determined by:

1. `?format=json` query parameter (takes precedence)
2. `Accept: application/json` header (when `text/html` is not also present)
3. Default: HTML

## Config Editing

Each authenticated user gets an independent editor session with its own working tree. Changes are tracked per-user and do not affect other users until committed.
<!-- source: internal/component/web/editor.go -- EditorManager, userSession -->

### Workflow

1. **Browse:** Navigate to a list (e.g., `/show/bgp/peer/`) to see entries in a table.
2. **Add:** Click `+ new` to create an entry. Fill in the name and unique fields. Values are validated against YANG types (e.g., IP addresses must be valid).
3. **Rename:** In table views, click the rename button to change an entry key. The new key keeps its case and literal bytes when the schema accepts them, and the existing subtree stays attached to the renamed entry.
4. **Edit:** Click an entry name to see its full config. Edit leaf values through inline fields.
5. **Review:** The commit bar at the bottom shows pending change count. Click "Review & Commit" to see a diff.
6. **Commit:** Apply changes. Conflicts with other users are detected and reported.
7. **Discard:** Click "Discard" to abandon your browser's pending changes. Separate SSH edits under the same username remain pending.
<!-- source: internal/component/web/editor.go -- Discard -->
<!-- source: internal/component/cli/editor_commit.go -- DiscardSessionPath -->

### Collections and Configuration Actions

Multi-word settings and leaf-lists show editable rows with Add, Remove, Move up,
Move down, Save and Clear controls. The schema decides which values and repeated
members are accepted. Leaf-lists also expose member activation; a multi-word
setting is activated or deactivated as a whole. Freeform blocks show entry-name
and value rows, and inline lists provide keyed entry links and an add form.

The **Configuration actions** panel offers copy and rename for a selected keyed
entry, whole-item activation, candidate validation, draft saving and saved
revision history. Restoring a revision changes the candidate, which can be
reviewed before committing. **Commit options** can apply a change with automatic
rollback; the owner of that pending commit can keep it or revert it from the
same panel. Read-only accounts can browse but have no mutation controls.

Open **Inspect** to view the candidate, running configuration or saved draft as
a tree or set commands. The separate candidate metadata controls show
authorship, active or inactive settings, your pending changes, all pending
changes, and validation issues. Compare the candidate with the running
configuration, saved draft, a saved revision or a user's changes. A revision
selection keeps its identity when a newer revision is saved.

**Editing sessions** lists session identifiers and pending-change counts.
Accounts with edit permission can disconnect another session; use Discard for
your own changes. **Display preferences** changes the shared editor's columns
and diagnostic presentation for your account. These inspection and preference
controls are available to read-only accounts.

Open **Inspect supplied configuration** to inspect a local file or pasted text
without loading it into your candidate. Choose the inspection, then select a
file or paste configuration. Comparison provides separate **Before** and
**After** inputs, each accepting a file or paste. The complete input is limited
to 195,840 bytes; a comparison shares that limit across both sources and their
framing. Read-only accounts can use these controls when their command policy
allows the inspection. Results and refusals appear below the form; the selected
file and pasted text remain available for correction.

Comparison reports every change in the two configurations, matching the
local file comparison: BGP settings as resolved values, every other setting as
written, including `system host`, and schema-declared ordered-list ordering.
Secrets are masked.
<!-- source: internal/component/config/infra/diff.go -- DiffConfigs -->

<!-- source: internal/component/web/component_graphical_supplied.templ -- graphicalSuppliedPanel, graphicalSuppliedInputs -->
<!-- source: internal/component/web/graphical_supplied.go -- HandleSuppliedInspection -->
<!-- source: internal/component/config/cli/cmd_diff.go -- parseAndResolve -->
<!-- source: internal/component/config/infra/diff.go -- DiffConfigs, diffListOrders -->

In **Edit**, open **Import into candidate** to select a local file or paste
configuration. Choose **Configuration root** for a complete configuration or
**Selected item** for a body relative to the item you are viewing. **Merge**
retains omitted settings; **Replace** removes settings omitted from that scope.
The request is limited to 4 MiB, including form fields and file headers.
An import changes only your candidate. Review its changes, then commit to apply
them. A refused import keeps both the candidate and your form available for
correction. Read-only accounts have no import controls.

<!-- source: internal/component/web/component_graphical_import.templ -- graphicalImportPanel -->
<!-- source: internal/component/web/graphical_import.go -- HandleGraphicalImport -->

<!-- source: internal/component/web/component_graphical_values.templ -- graphicalValues, graphicalInlineList -->
<!-- source: internal/component/web/component_graphical_actions.templ -- graphicalActionPanel -->
<!-- source: internal/component/web/graphical_actions.go -- HandleGraphicalAction, rollbackGraphicalRevision -->

### Validation

Field values are validated server-side against YANG types before being accepted:
<!-- source: internal/component/config/schema.go -- ValidateValue -->

Every value set from the web (a form field, the add form or browser CLI
`set`) is then judged with the checks a config file gets: the
enumeration, range, length and pattern the YANG declares, a union's members,
and each member of a leaf-list. A value outside them is refused when it is set,
not at commit. The refusal names the setting path and the value, lists the
accepted values (each with its one-line help) or the accepted bounds or format,
and ends with the command that fixes it:

```text
bgp peer peer1 session capability route-refresh: "requre" is not an accepted value (did you mean require?)
route-refresh accepts only these values:
  enable   Advertise the capability; no enforcement
  disable  Do not advertise the capability; no enforcement
  require  Advertise the capability; refuse the session when the peer does not offer it
  refuse   Do not advertise the capability; refuse the session when the peer offers it
fix: set bgp peer peer1 session capability route-refresh require
```

The same refusal reads the same on every surface; the range, length, pattern
and union forms are in
[Value refusals](https://github.com/ze-software/ze/blob/main/docs/architecture/config/yang-config-design.md#value-refusals).

A form or the commit modal holds no editor position, so its fix is the `set`
from the root. The browser CLI frames the fix for the position the
line was typed at: the `set` as typed, corrected, for a refused `set`, and
`top set <full path> ...` for a commit refusal inside a context. A new list key
is judged where its entry is created, and the add form judges every field
before it creates the entry, so a refused field never leaves a partial entry.
A key refused by the terminal's `rename`, `copy`, `insert`, `activate` or
`deactivate` gets that same command as its fix, with the key corrected and
framed for the position, and the Rename button's refusal ends with
`fix: rename <full path> to <corrected key>`.
<!-- source: internal/component/web/editor.go -- EditorManager.SetValue, judgeValue -->
<!-- source: internal/component/web/cli_editor.go -- executeTerminalEditorCommand -->
<!-- source: internal/component/web/handler_config_entry.go -- HandleConfigRenameWithAuthorizer -->
<!-- source: internal/component/cli/model_commands_edit.go -- cmdSet -->
<!-- source: internal/component/web/handler_config_entry.go -- addFieldSetPath -->

Fix commands quote list keys and values that contain spaces or quotes. Type
the whole printed command, including its quotes. A refused key can precede a
secret assignment: its correction keeps the assignment and masks the secret.
<!-- source: internal/component/cli/fix_frame.go -- FrameFix -->
<!-- source: internal/component/cli/fix_frame.go -- FrameFix -->

| Type | Validation |
|------|-----------|
| IP address | Must be a valid IPv4 or IPv6 address |
| IPv4 | Must be a valid IPv4 address |
| IPv6 | Must be a valid IPv6 address |
| Prefix | Must be a valid CIDR prefix |
| Uint16/Uint32 | Must be a valid unsigned integer in range |
| Boolean | Normalized to `true`/`false` |

YANG `unique` constraints are enforced: duplicate values are rejected with an error naming the conflicting entry.
<!-- source: internal/component/web/handler_config_walk.go -- checkUniqueConstraint, validateUniqueOnSet -->

Add and rename preserve entry key case and literal bytes accepted by the
schema. Generated links encode each key separately, so a space or slash inside
a key remains part of that key when the operator follows it or saves a field.
<!-- source: internal/component/web/path_transport.go -- encodeConfigPath, decodeConfigPath -->
<!-- source: internal/component/web/handler_config_entry.go -- HandleConfigAddWithAuthorizer, HandleConfigRenameWithAuthorizer -->

Duplicate entry keys are rejected. Validation runs before the entry is created, so invalid input never produces a partial entry.

Navigating to a non-existent list entry (e.g., `/show/bgp/peer/london/` when `london` has not been created) redirects to the root view with an error notification.
<!-- source: internal/component/web/fragment.go -- HandleFragment, isListEntryPath check -->

A field check reads one value. A commit runs the whole-config validator, the same one `ze config validate` runs, over the config the commit is about to write. This includes the BGP peer pipeline, so a peer the daemon would refuse blocks the commit while the config is still on the screen. The refusal appears in the commit modal as `Commit failed:` with the validator's message, and nothing is written.
<!-- source: internal/component/cli/editor_commit.go -- validateStagedTree -->
<!-- source: internal/component/web/handler_config_commit.go -- handleCommitPost -->

### Notifications

Error notifications appear as toasts in the top-right corner with a 30-second countdown. Click the countdown to pause (for screenshots). Click the close button to dismiss immediately.
<!-- source: internal/component/web/component_notification_error.templ -->

A refused action raises a toast carrying the status and the message the daemon wrote, and the same message lands in the field or panel the action came from. A request that gets no answer at all raises a toast that says so instead: the daemon is unreachable, and nothing was changed.
<!-- source: internal/component/web/assets/notification.js -- handleResponseError and handleRequestError -->

### Input Auto-Save

Text and number fields save on blur, Enter and a one-second debounce after
typing. A save issued while that same field's request is in flight is dropped;
the response replaces the field with the accepted candidate value. Enter saves
the field, and the separate commit step applies the candidate configuration.

Enter arrives as `ze-enter`, a named event a delegated listener in `assets/cli.js` dispatches. The listener reads the element's own `hx-trigger`, lives in a file rather than in an attribute, and survives every swap. An inline editor replaces itself, so each input also carries a stable id derived from its leaf path: without one, htmx restored focus by looking up the empty string and the caret was lost on every save.
<!-- source: internal/component/web/input_text.templ -- hx-trigger="blur changed, ze-enter, input changed delay:1s" -->
<!-- source: internal/component/web/assets/cli.js -- initEnterSubmit -->
<!-- source: internal/component/web/view.go -- fieldInputID, escapeFieldID, fieldSaveSync -->

### Conflict Detection

When two users edit the same leaf concurrently, the commit reports which paths conflict, showing both the local and other user's values. The user must resolve conflicts before committing.
<!-- source: internal/component/web/handler_config_commit.go -- handleCommitPost -->
<!-- source: internal/component/cli/contract/contract.go -- CommitResult.Conflicts, Conflict -->

### Session Limits

The editor manager allows up to 50 concurrent user sessions. Idle sessions (no activity for 1 hour) are evicted when capacity is reached.
<!-- source: internal/component/web/editor.go -- NewEditorManager, maxSessions, idleTimeout -->

Reopening an evicted editor or reconnecting after a daemon restart restores
your saved and unsaved web changes. Review, pending counts, save and commit
refer to those resumed changes. Other users' changes and your SSH sessions
remain separate. If the stored intent cannot be read or recovery cannot finish,
the page refuses to open; repair the reported problem, then reload `/show/`.
<!-- source: internal/component/cli/editor_resume.go -- ResumeSession -->

## Browser CLI

The **CLI** link opens the dedicated `/cli` page. Its prompt shows the current
configuration position, for example `ze[bgp peer london]#`. Commands execute in
order, so a command following `edit` uses the position returned by that edit.
The page displays command output and feedback separately; the up and down keys
recall command history when the completion menu is closed.

**Download** saves the current successful command result to your computer, with
its selected output format. The download uses the answer already received and
runs no second command. Complete finite and raw streamed results up to 4 MiB
are supported, including older streamed bytes removed from the visible tail.
The server must confirm successful stream completion before Download becomes
available. Failure, Stop, missing completion or an exceeded limit never offers a
partial capture as a complete file. Starting another command clears the previous
download. Unfinished confirmation or load requests offer no download.
Interactive monitors offer a **Current frame snapshot**, which saves only the
currently displayed frame. A command containing `| save` still cannot write a
path on the daemon.

The authenticated history API is `GET /cli?history=1&mode=config` (or
`mode=operational`). It returns a bounded, masked `entries` array for the signed-in
user, and reports an unavailable store as an error. Commands submitted through
the command bar, terminal and stream endpoint share this saved history producer.
Editor discard preserves recall. A terminal response carries `history-warning`
when recall could not be updated; the command can still execute. Uploaded file
and pasted configuration content never become recall entries.
<!-- source: internal/component/web/cli_history.go -- recordCLIHistory, writeCLIHistory -->

Recall survives reloading the page and stays separate for each account and CLI
mode. Press Up with the completion menu closed to read the latest saved recall.
Press Down past the newest entry to restore your unfinished input. If recall is
unavailable, the page displays the failure and still accepts commands.
<!-- source: internal/component/web/assets/cli.js -- commandRecall, initTerminal -->

Configuration commands use the same editor producers as the SSH CLI. Commit and
discard use the web session's shared publication and confirmed-commit controls.
Operational commands and live monitors require the running daemon; `--web-only`
provides configuration editing without an operational command service.

| Command | Effect |
|---------|--------|
| `edit <path>`, `up`, `top` | Change the configuration position |
| `set <path> <value>`, `delete <path> [member]` | Change a schema-backed setting relative to the current position; quoted keys and values retain their token boundaries |
| `copy <entry> to <key>`, `rename <entry> to <key>` | Copy or rename a keyed list entry |
| `activate <path>`, `deactivate <path>` | Change activation of a setting, a supported leaf-list member, a container or a list entry |
| `show [path]` | Display candidate configuration at the current or supplied path |
| `show confirmed` | Display the committed configuration |
| `show \| compare` | Review candidate changes |
| `errors` | Display validation and reload issues, or `No issues` |
| `save`, `history`, `rollback <revision>` | Save the draft, list saved revisions or restore one into the candidate |
| `top <command>` | Run one command at the root; the position stays where it was unless that command moves it |
| `commit now`, `commit confirmed <seconds>`, `commit accept`, `commit abort`, `commit verify` | Apply, confirm, revert or validate through the shared commit machinery; `commit` alone lists the accepted subcommands |
| `discard [all\|<path>]` | Drop all candidate changes or only changes under the supplied path |
| `run <command>` | Run an operational command while keeping the configuration position |
| `exit`, `configure` | Leave configuration mode or enter it; operational `exit` returns to the graphical editor |
| `quit` | Return to the graphical editor |
| `clear` | Restore the current candidate view in configuration mode, or clear operational output |
| `stop`, `restart` | Ask for explicit confirmation, then submit the daemon's authorized shutdown or restart operation |
| `help` | Display the available commands |

The [config editor guide](../config-editor/index.md) describes the command grammar and
commit conflict handling. An explicit `force` option can override warnings and
conflicting drafts where the shared commit command permits it.

Leaving with pending changes asks for confirmation and keeps your candidate.
Clearing the display also keeps those changes and your current position.
Configuration-mode `top clear` and `run clear` have the same effect as `clear`.
Help in both modes uses the CLI's shared command catalog.
The confirmation appears beside the terminal input with **Confirm** and
**Cancel** buttons. Cancel keeps you in the current mode with your output and
candidate unchanged.

Daemon `stop` and `restart` affect every connected user. They always ask for
confirmation and still require permission through the normal command service.
`top stop` and `top restart` have the same effect from configuration mode.
These actions keep your candidate and are separate from clearing the display.

<!-- source: internal/component/cli/contract/lifecycle.go -- LifecycleCommand -->

<!-- source: internal/component/web/cli_controls.go -- executeTerminalControl, terminalEditorHelp -->

<!-- source: internal/component/web/cli_terminal.go -- executeTerminalCommand, executeTerminalConfigMode -->
<!-- source: internal/component/web/cli_editor.go -- executeTerminalEditorCommand -->
<!-- source: internal/component/cli/editor_transport.go -- ExecuteConfigCommand -->
<!-- source: internal/component/cli/model_commands_commit.go -- cmdErrors -->

### Loading Configuration

`load terminal absolute|relative merge|replace` opens a paste form. A `load file`
command opens a browser file chooser; its filename labels the input rather than
reading a file from the daemon's filesystem. Both load paths accept at most
4 MiB and stage the parsed changes in the candidate, ready for review and commit.
A relative load takes a hierarchical body for the current position; absolute
loads can also use set-format statements.

<!-- source: internal/component/web/assets/cli.js -- initTerminal -->
<!-- source: internal/component/cli/editor_load.go -- ParseLoad, LoadMerge, LoadReplace -->

For read-only inspection, operational commands under `show config supplied`
accept `content <base64>` for dump, validation, formatting, dependency graphs
and comparison. These inspect the supplied bytes without loading them into
your candidate. Single-source content is the native configuration; comparison
content is one JSON object with `before` and `after` configuration strings,
encoded together. The decoded input is limited to 195,840 bytes. See the
[command reference](../command-reference/index.md#conventions) for labels and selections.
The graphical **Inspect supplied configuration** controls compose these inputs
for you, so no manual base64 encoding is needed when using those forms.
<!-- source: internal/component/config/cli/register.go -- registerSuppliedCommands -->
<!-- source: internal/component/config/cli/config_supplied.go -- suppliedHandler, suppliedPair -->
<!-- source: internal/plugins/config-cli/yang/ze-config-cli-cmd.yang -- supplied-content -->

### Completion and Output Format

Tab offers schema and command completions for the terminal's current position.
Arrow keys or Tab select an item and Enter accepts it. An unquoted `?` explains
the selected candidate or completed command using its declared long help; a
quoted `?` remains data. Tab also reveals long help once no completion remains.
Escape closes the explanation before closing a menu beneath it. Enum values
declared in YANG, including enum members of a union, are offered by the shared
completer. A failed runtime query displays its reason and correction, so it
cannot be mistaken for an empty set of values.

<!-- source: internal/component/web/assets/cli.js -- completionResponse, initTerminal -->

In configuration mode, `top set` completes paths from the root, even when your
current position is nested. `top run` completes operational commands and live
argument values as `run` does. Repeated `top` prefixes keep that root selection.
<!-- source: internal/component/web/cli.go -- completeCLIInput, explainCLIInput -->

In operational mode, `set cli format` queries the output preference and
`set cli format json` selects a format for this authenticated user's session.
The preference also applies to config-mode `run` commands; an explicit format
pipe overrides it for one command. The configured process default remains
unchanged, and raw live streams retain their existing text format.

<!-- source: internal/component/web/cli.go -- HandleCLIComplete -->
<!-- source: internal/component/cli/session_format.go -- ExecuteSessionFormat, SessionFormatCompletions -->
<!-- source: internal/component/web/cli_format.go -- sessionFormatCommand, executeTerminalOperationalForUser -->

### Live Commands

Registered interactive monitors show the same live view as the CLI, with buttons
for the actions that view offers. For `run monitor bgp`, use **Next sort column**
and **Reverse order** to sort, **Previous peer** or **Next peer** to select a row,
**Peer detail** to inspect it and **Back to peers** to return. **Help** shows the
view's own explanation. The highlighted row, detail and updates come from the
same producer as the terminal dashboard.

Other registered monitoring commands stream successive daemon answers into the page.
**Stop** or Ctrl+C ends the stream, and submitting a new command stops the active
stream before running the next command. Leaving the page also cancels it. The
browser removes ended controls and shows why a failed action or producer could
not continue. Raw streams are bounded at 4 MiB by the server; the browser keeps a
bounded visible tail and marks older output that it has removed. A completed
download retains the original bytes, including those no longer visible.
Interactive views retain only the latest frame, limited to 64 KiB. Live commands
accept the same stream pipes as the CLI, such as `| json`, `| log` and
`| match <text>`. Invalid chains name the refused pipe. Remote `| save <path>`
is refused; use Download to save results on your computer. A raw piped event
larger than 4 MiB stops the stream with a reason.

<!-- source: internal/component/web/cli_stream.go -- HandleCLITerminalStream, cliStreamWriter.Write -->
<!-- source: cmd/ze/hub/server_stream.go -- serverStreamDispatcher -->
<!-- source: internal/component/command/pipe_stream.go -- RunStreamPipes -->
<!-- source: internal/component/web/assets/cli.js -- initTerminal -->

## Live Updates

The web interface uses Server-Sent Events (SSE) to notify connected browsers when configuration changes are committed by any user. A notification banner appears with the username and a "Refresh" button.
<!-- source: internal/component/web/sse.go -- EventBroker, BroadcastConfigChange, SendTo -->

The same stream carries, to each user alone, what the SSH editor's status
line would show that user: the countdown of a `commit confirmed` window, or
whose window is pending, how a window ended, and that another user's forced
commit discarded one of your changes. They arrive as `config-change` events,
so they use the same banner. While a user's own commit is in flight, the
window news for that user waits, so a user's own `commit accept` or
`commit abort` is never reported as closed by another session. See "Commit
Confirmed" in `config-editor.md`.
<!-- source: internal/component/web/window_notices.go -- WindowNotices, pollWindow -->

Connect to the SSE stream at `/events` (requires authentication). Each stream is registered under its user. The broker supports up to 100 concurrent SSE clients. Slow clients that fall behind have events dropped rather than blocking other clients.

### Event Format

Events use the standard SSE wire format:

```
event: config-change
data: <html-fragment>
```

The HTML fragment contains a notification banner with the change description and action buttons.

## Admin Commands

The `/admin/` tier provides a browsable tree of administrative commands. Container nodes display navigable links to sub-commands. Leaf commands display a parameter form with an "Execute" button.

The form also shows the command's help, read from the same command model every other surface reads. The declared one-line summary is the lede above the form, and the declared long explanation is the paragraph under it. A command that declares no explanation shows the summary alone. A path the command tree does not hold shows neither text, and never borrows its parent's.
<!-- source: internal/component/web/handler_admin.go -- HandleAdminView, HandleAdminExecute, buildAdminFragmentData -->
<!-- source: internal/component/web/component_command_form.templ -- commandForm -->

Admin command results are displayed as titled cards showing the command name, output text, and success/error styling.

## Resilience

**Corrupt change files:** An unknown path, malformed command or invalid
structural syntax in a stored per-user change file prevents the browser from
reopening that candidate. The error names the file, which remains unchanged.
Normal web discard also stops when it cannot read the audit snapshot. Open
`/config/recover` with an account authorized to discard configuration changes.
Review the warning, select the confirmation checkbox, and press **Discard
unreadable pending change file** to discard that account's corrupt change file.
This also discards SSH or console changes in the same file: their ownership
cannot be read. The file can retain structural instructions after **Save draft**;
those instructions are lost too. The separate saved draft file, other accounts
and running configuration are kept. Cancel leaves the corrupt file intact. If the file changes
after the form opens, recovery refuses; open the form again to review it.
An invalid leaf value or leaf-list member
in an otherwise readable API candidate remains editable: commit refuses the
value, and the operator can apply its proposed correction while preserving
other changes.
<!-- source: internal/component/cli/editor_draft.go -- readChangeFile -->
<!-- source: internal/component/cli/editor_change_recovery.go -- InspectCorruptChangeFile, CorruptChangeFile.Discard -->
<!-- source: internal/component/web/component_config_recovery.templ -- recovery confirmation -->

**Asset caching:** Static assets (`/assets/`) are served with `Cache-Control: no-cache, must-revalidate` so browsers always pick up changes after binary updates without requiring a hard refresh.
<!-- source: internal/component/web/render.go -- AssetHandler -->
