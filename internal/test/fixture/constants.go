// Design: docs/architecture/testing/ci-format.md -- the compiled fixture API
// Detail: one constant for each string literal this package repeats, named for
// the value it carries rather than for its Go type.
// Detail: a literal that means two unrelated things here gets one constant for
// each meaning, so the text repeats and the names do not. Each such constant
// states its meaning on its own line.
// Related: fixture.go holds the shared fixture helpers.

package fixture

import (
	"github.com/ze-software/ze/internal/core/family"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

// Command words and fields several le fixtures share.
const (
	areaSpec            = "spec"
	containerConnection = "connection" // The BGP peer config container of the local and remote endpoints.
	fieldDryRun         = "dry-run"
	flagJSON            = "--json"
	leafPort            = "port"   // The config leaf that carries a listener or peer port.
	leafTarget          = "target" // The acme-monitor leaf the external YANG fixture plugin's module defines.
	mockRadius          = "radius" // The le mock server selector.
	programZeLe         = "ze le"
	sourceRadius        = "radius" // The authentication source recorded after a RADIUS login.
)

// Environment variables and build tags.
const (
	binaryLE           = "le"
	buildTagLE         = repofeaturetags.LEBase
	envCGOEnabled      = "CGO_ENABLED"
	envCLIFormat       = "ze.cli.format"
	envConfigDir       = "ZE_CONFIG_DIR"
	envConfigDirDotted = "ze.config.dir"
	envGitAuthorDate   = "GIT_AUTHOR_DATE"
	envGitAuthorEmail  = "GIT_AUTHOR_EMAIL"
	envGitAuthorName   = "GIT_AUTHOR_NAME"
	envGitCommitDate   = "GIT_COMMITTER_DATE"
	envGitCommitEmail  = "GIT_COMMITTER_EMAIL"
	envGitCommitName   = "GIT_COMMITTER_NAME"
	envGitConfigGlobal = "GIT_CONFIG_GLOBAL"
	envGitConfigSystem = "GIT_CONFIG_NOSYSTEM"
	envGoCache         = "GOCACHE"
	envHome            = "HOME"
	envLEBuildName     = "ZE_LE_BUILD_NAME"
	envLogBFD          = "ze.log.bfd"
	envLogBGP          = "ze.log.bgp"
	envLogInterface    = "ze.log.interface"
	envLogVPP          = "ze.log.vpp"
	envNoColor         = "NO_COLOR"
	envPath            = "PATH"
	envReadyFile       = "ZE_READY_FILE"
	envRepoRoot        = "ZE_REPO_ROOT"
	envSSHEphemeral    = "ZE_SSH_EPHEMERAL"
	envSSHHost         = "ZE_SSH_HOST"
	envSSHPassword     = "ZE_SSH_PASSWORD"
	envSSHPort         = "ZE_SSH_PORT"
	envSSHUsername     = "ZE_SSH_USERNAME"
	envTerm            = "TERM"
	envTestBGPPort     = "ze_test_bgp_port"
	envTypeString      = "string" // The value type an env.EnvEntry declares.
	envXDGRuntimeDir   = "XDG_RUNTIME_DIR"
)

// The two address families these fixtures drive, in the registry's own words.
// family.MustRegister composed each name from its AFI and SAFI parts
// (internal/core/family/registry.go), so a fixture never spells one itself.
var (
	familyIPv4Unicast = family.IPv4Unicast.String()
	familyIPv6Unicast = family.IPv6Unicast.String()
)

// Addresses, prefixes, ports and other network identities.
const (
	addrLoopback = "127.0.0.1"
	// addrLGPeerFirst and addrLGPeerSecond are the pseudo-peers the looking
	// glass scenarios inject routes under; injectRoute09 makes each route's
	// next hop its peer.
	addrLGPeerFirst  = "10.0.1.1"
	addrLGPeerSecond = "10.0.1.2"
	// prefixTen8, prefixTen1Of16 and prefixDefaultIPv4 are the nested IPv4
	// prefixes the LLGR and covering-lookup scenarios contest and look up.
	prefixTen8        = "10.0.0.0/8"
	prefixTen1Of16    = "10.1.0.0/16"
	prefixDefaultIPv4 = "0.0.0.0/0"
	// prefix30First is the first prefix the AS path search scenario injects.
	prefix30First = "10.30.1.0/24"
	// prefix192Short is the short covering prefix the covering-lookup scenario
	// looks up beside 192.0.0.0/24, whose text starts the same way.
	prefix192Short = "192.0.0.0/2"
	// prefix72First is the first prefix the 10.72 policy and reload
	// scenarios announce.
	prefix72First = "10.72.1.0/24"
	// prefix82First is the first prefix the RFC 8212 policy scenario
	// announces.
	prefix82First              = "10.82.1.0/24"
	addrLoopbackSecond         = "127.0.0.2"
	addrLoopbackThird          = "127.0.0.3"
	addrLoopbackV6             = "::1"
	addrPeerOne                = "10.0.0.1"
	addrPeerTwo                = "10.0.0.2"
	addrPeerThree              = "10.0.0.3"
	addrTestNet1First          = "192.0.2.1"
	addrTestNet1Second         = "192.0.2.2"
	addrTestNet2Nexthop        = "198.51.100.162"
	addrTestNet3Nine           = "203.0.113.9"
	configPathRouterID         = "bgp.router-id"
	hostnamePyPeer             = "py-peer"
	metricPeerStateTransitions = "ze_peer_state_transitions_total"
	metricPoolUsedRatio        = "ze_bgp_pool_used_ratio"
	peerNameOne                = "peer1"
	peerNameSource             = "source" // The configured BGP peer a scenario sends its routes from.
	portL2TP                   = "1701"
	prefixTestNet1             = "192.0.2.0/24" // RFC 5737 TEST-NET-1.
	prefixTestNet2             = "198.51.100.0/24"
	prefixTenNinetyNine        = "10.99.0.0/24"
	prefixTenOne               = "10.1.0.0/24"
	prefixTenTwenty            = "10.20.0.0/24"
	prefixTenZero              = "10.0.0.0/24"
	suffixHostV4               = "/32"  // Appended to an IPv4 address, the prefix of that host alone.
	suffixHostV6               = "/128" // Appended to an IPv6 address, the prefix of that host alone.
)

// Plugin dispatch statuses, event kinds and payload field keys.
const (
	codeDoctorTestCacheDown = "doctor-test-cache-down"
	decoderKindEnd          = "end"
	decoderKindNay          = "nay"
	decoderKindTop          = "top" // The kind of a decoded line.
	directionExport         = "export"
	directionLocal          = "local"
	directionReceived       = "received"
	directionRemote         = "remote"
	directionSent           = "sent"
	eventActionDel          = "del"    // The action of an update event's NLRI entry that withdraws it.
	eventSignal             = "signal" // The event kind a plugin subscribes to, and the message type it arrives as.
	eventState              = "state"  // The event kind a plugin subscribes to.
	eventUpdate             = "update" // The event kind a plugin subscribes to.
	fieldASPath             = "as-path"
	fieldAddress            = "address" // The peer address field in a payload.
	fieldChecks             = "checks"
	fieldCommand            = "command" // The field in a payload or a query.
	fieldCommands           = "commands"
	fieldEvidence           = "evidence" // The RFC check report field.
	fieldCommunity          = "community"
	fieldContainer          = "container"
	fieldContent            = "content"
	fieldCount              = "count" // The field in a payload.
	fieldEntries            = "entries"
	fieldError              = "error" // The field that carries the message in a payload.
	fieldErrors             = "errors"
	fieldFamily             = "family"
	fieldFile               = "file"
	fieldFiles              = "files"
	fieldImage              = "image"
	fieldInput              = "input" // The field in a payload.
	fieldKind               = "kind"
	fieldLeaf               = "leaf"
	fieldLocalAS            = "local-as"
	fieldLocalPreference    = "local-preference" // The LOCAL_PREF field name in the filter protocol.
	fieldMED                = "med"              // The MED attribute name in the filter and catalog protocols.
	fieldMessage            = "message"
	fieldMode               = "mode"
	fieldOrigin             = "origin" // The ORIGIN field name in the filter protocol.
	fieldName               = "name"   // The field in a payload.
	fieldPackages           = "packages"
	fieldPassed             = "passed"
	fieldPath               = "path"
	fieldPeer               = "peer"  // The field in a payload.
	fieldPeers              = "peers" // The field in a payload.
	fieldPeersConfigured    = "peers-configured"
	fieldPeersEstablished   = "peers-established"
	fieldReady              = "ready"
	fieldRecord             = "record" // The rejected answer's record number.
	fieldRerun              = "rerun"
	fieldResult             = "result" // The field in a dispatch answer.
	fieldRouterID           = "router-id"
	fieldRows               = "rows"
	fieldSessionID          = "session-id" // The L2TP session identifier in a payload or a diagnostic argument.
	fieldSeverity           = "severity"   // The severity field of a diagnostic, a doctor check or a finding.
	fieldSource             = "source"     // The field in a payload.
	fieldStage              = "stage"
	fieldState              = "state" // The peer or interface field in a payload.
	fieldStatus             = "status"
	fieldTimeout            = "timeout"
	fieldTop                = "top" // The field in a payload.
	fieldTotals             = "totals"
	fieldTree               = "tree" // The checkout path in an evidence report.
	fieldTunnelID           = "tunnel-id"
	fieldType               = "type"    // The field in a payload.
	fieldUpdated            = "updated" // The field in a payload.
	fieldVRPCount           = "vrp-count"
	fieldValue              = "value"
	fieldWritten            = "written"
	linkStateDown           = "down"
	messageTypeUpdateRPKI   = "update-rpki"
	namespaceBGP            = "bgp"
	namespaceL2TP           = "l2tp" // The event namespace an L2TP observer subscribes to.
	outcomeFailed           = "failed"
	outcomeSuccess          = "success"
	severityError           = "error" // The severity of a diagnostic entry.
	severityWarning         = "warning"
	sourceCommand           = "command" // The origin recorded for a config node.
	sourceConfig            = "config"  // The origin recorded for a config node.
	sourceGroup             = "group"   // The origin recorded for a config node.
	sourcePeer              = "peer"    // The origin recorded for a config node.
	stateActive             = "active"  // The BGP peer state while it waits to accept or open its connection.
	stateEstablished        = "established"
	stateNotSynchronized    = "not-synchronized"
	statusCommitted         = "committed"
	statusDone              = "done"
	statusError             = "error" // The status a dispatch returns.
	statusStale             = "stale"
	statusUpdated           = "updated" // The status a config write returns.
	// stepLocalPreference is the best-path selection step a higher LOCAL_PREF
	// decides, as the "step" key of a best-path decision step names it.
	stepLocalPreference = "local-preference"
	verdictInvalid      = "invalid"
	verdictPass         = "pass"
	verdictValid        = "valid"
)

// Table columns, render operators, pipe operators and answer shapes.
const (
	aliasPeers               = "peers"   // The name of a declared pipe alias.
	aliasSummary             = "summary" // The name of a declared pipe alias.
	columnAddress            = "address" // The column in a rendered table.
	columnConnectionsDropped = "connections-dropped"
	columnGroup              = "group" // The column in a rendered table.
	columnName               = "name"  // The column in a rendered table.
	columnNextHop            = "next-hop"
	columnOrigin             = "origin" // The BGP origin column of a route row.
	columnPrefix             = "prefix"
	columnRemoteAS           = "remote-as"
	columnReceive            = "receive" // The direction column of a paths-limit table.
	columnSend               = "send"    // The direction column of a paths-limit table.
	columnState              = "state"   // The column in a rendered table.
	columnUptime             = "uptime"
	headerContentType        = "Content-Type"
	mediaTypeJSON            = "application/json"
	pemCertificate           = "CERTIFICATE"
	pipeCommand              = "pipe"  // The standalone pipe command.
	pipeCount                = "count" // The pipe operator.
	pipeFirst                = "first" // The pipe operator.
	pipeOrigin               = "origin"
	pipeResolve              = "resolve"
	renderJSON               = "json"
	renderRaw                = "raw"
	renderTable              = "table" // The pipe operator and the answer shape.
	renderText               = "text"
	renderYAML               = "yaml"
	shapeDoc                 = "doc"
	shapeIPAddress           = "IP address"
	shapeMap                 = "map"
	shapeOneDocument         = "one document"
	shapeTab                 = "tab"
)

// Command words and whole commands, for ze, ip, nft and the shell.
const (
	argAdd                       = "add"
	argCommand                   = "command" // The command keyword.
	argCommit                    = "commit"  // The git subcommand, and the `le` area of the same name.
	argConfig                    = "config"  // The git subcommand.
	argDel                       = "del"     // The ip subcommand verb that removes, beside argAdd.
	argInit                      = "init"
	argInterface                 = "interface"
	argName                      = "name" // The command keyword.
	argQuiet                     = "--quiet"
	argShow                      = "show"
	argType                      = "type" // The command keyword.
	cmdAnnounceFirstPrefix       = "update text nhop 101.1.101.1 nlri add 1.1.0.0/24"
	cmdDocIndex                  = "doc index"
	cmdRIBInject                 = "request bgp rib inject " // Followed by the peer, the family and the route.
	cmdShowBGP                   = "show bgp"
	cmdShowBGPPeerList           = "show bgp peer list"
	cmdShowNsaliasCounters       = "show nsalias counters"
	cmdShowPipealiasCounters     = "show pipealias counters"
	cmdShowPipehelpCounters      = "show pipehelp counters"
	cmdShowRIBCandidatesPrefix   = "show rib candidates prefix " // Followed by the prefix.
	cmdShowTestCompletionHidden  = "show test-completion hidden"
	cmdShowTestCompletionVisible = "show test-completion visible"
	cmdShowTestTwoTextsBoth      = "show test-twotexts both"
	cmdShowTestTwoTextsSummary   = "show test-twotexts summary"
	cmdShowVersion               = "show version"
	cmdWatchdogAnnounceDNSR      = "request bgp watchdog announce dnsr"
	cmdWatchdogWithdrawDNSR      = "request bgp watchdog withdraw dnsr"
	flagFragment                 = "--fragment"
	flagHelp                     = "--help"
	ipObjectAddress              = "address" // The ip object being configured.
	ipObjectLink                 = "link"
	ipObjectRoute                = "route" // The ip object being configured.
	ipTypeDummy                  = "dummy"
	ipWordDev                    = "dev"
	ipWordMetric                 = "metric"
	ipWordProto                  = "proto"
	ipWordSet                    = "set"
	ipWordVia                    = "via"
	nftChain                     = "chain"
	nftChainFlowspecForward      = "flowspec-fwd"
	nftChainForward              = "forward"
	nftChainInput                = "input" // The nftables chain.
	nftFamilyInet                = "inet"
	nftForwardHookSpec           = "{ type filter hook forward priority -1 ; policy accept ; }"
	nftMatchDestination          = "daddr"
	nftRule                      = "rule"
	nftTable                     = "table" // The nftables object.
	nftTableAnomalyShape         = "anomaly-shape"
	nftTableFlowspec             = "flowspec"
	nftTableSurfprotect          = "surfprotect"
	nftVerdictAccept             = "accept" // The nftables verdict.
	nftVerdictDrop               = "drop"
	programQEMUAArch64           = "qemu-system-aarch64"
	programQEMUX8664             = "qemu-system-x86_64"
	programSSH                   = "ssh"
	programQEMU                  = "qemu" // The emulator program a fake records.
	xfrmWordSource               = "src"  // The ip xfrm output column.
)

// le areas, actions, checks and gates.
const (
	actionAccept                  = "accept" // The policy decision recorded for a route.
	actionCheck                   = "check"
	actionCreate                  = "create"
	actionL2TPTest                = "l2tp"
	actionPayloadReport           = "report payload"
	actionRecord                  = "record" // The le action that records evidence.
	actionReject                  = "reject"
	actionReleaseCandidate        = "release-candidate"
	actionReload                  = "reload"
	actionReport                  = "report"
	actionRouterReport            = "report router"
	actionRun                     = "run"
	actionSelftest                = "selftest"
	actionStart                   = "start"
	actionUpdate                  = "update" // The le action verb.
	actionVPPHugepagesTest        = "vpp-hugepages"
	actionVPPIfaceTest            = "vpp-iface"
	actionVPPTest                 = "vpp"
	areaCLI                       = "cli"
	areaDeployment                = "test deployment"
	areaDigest                    = "ai digest"
	areaAIRules                   = "ai rules"
	areaEvidence                  = "verify evidence"
	areaQEMU                      = "test qemu" // The le area.
	areaRepository                = "repo"
	areaRFC                       = "rfc" // The le RFC command area.
	areaRules                     = "rules"
	areaTest                      = "test" // The le test area.
	areaTier                      = "arch tier"
	checkCIDispatch               = "cli dispatch"
	checkCLIGrammar               = "cli grammar"
	checkCommandOwnership         = "cli ownership"
	checkConfigClaims             = "config claims"
	checkConfigCoercion           = "config coercion"
	checkDashStdio                = "cli stdio"
	checkDocWiring                = "doc wiring"
	checkFSPersistence            = "arch fs-persistence"
	checkGoVersion                = "go version-pin"
	checkIfaceResolution          = "arch iface-resolution"
	checkPluginBoundary           = "plugin boundary"
	checkPortDefaults             = "config ports"
	checkRepoCompiles             = "repo compiles"
	checkRepositoryTree           = "tree" // The repo check subject.
	checkStaticcheckFeatureMatrix = "go staticcheck"
	checkTestSensitivity          = "test sensitivity"
	checkYANGLeafMentions         = "config unread-leaves"
	gateGokrazyGosum              = "build gosum"
	gateProtocolSkeleton          = "rfc skeletons"
)

// Files, paths and expected text.
const (
	contentCoreFeatureGate          = "ze_core\n"
	contentEmptyMain                = "package main\n\nfunc main() {}\n"
	contentFeatureGate              = "fixture\n"
	contentGitIgnoreTmp             = "tmp/\n"
	descriptionCountersAlone        = "The counters alone"
	dirBuilder                      = "builder"
	dirCommon                       = "common"
	dirOut                          = "out"
	dirPlan                         = "plan"
	dirSource                       = "src" // The kernel source directory.
	valueNone                       = "none"
	expansionVRPCount               = "display kind vrp-count"
	fileBGPConf                     = "ze-bgp.conf"
	fileConfig2Conf                 = "config2.conf"
	fileCoreMD                      = "CORE.md"
	fileDaemonPID                   = "daemon.pid"
	fileDaemonReady                 = "daemon.ready"
	fileDatabaseZefs                = "database.zefs" // The zefs blob a storage scenario writes or refuses.
	fileFeatureGates                = "feature-gates.txt"
	fileGitIgnore                   = ".gitignore"
	fileGoMod                       = "go.mod"
	fileSampleGo                    = "sample.go"
	fileTriggersMD                  = "TRIGGERS.md"
	fileZeMain                      = "cmd/ze/main.go"
	gitBranchMain                   = "main" // The branch a scratch repository pushes.
	gitCommitNoSign                 = "commit.gpgsign=false"
	gitFixtureEmail                 = "fixture@example.invalid"
	gitFixtureName                  = "Fixture"
	gitFixtureTimestamp             = "2000-01-02T12:00:00Z"
	gitInitFixtureBranch            = "--initial-branch=fixture"
	gitInitNoTemplate               = "--template="
	gitRemoteOrigin                 = "origin" // The git remote name, not the BGP attribute.
	logBFDConfigured                = "bfd plugin configured"
	logBFDPinnedSessionCreated      = "bfd pinned session created"
	logBFDRunning                   = "bfd plugin running"
	logBFDSessionOpenedForPeer      = "bfd session opened for peer"
	logBFDStarting                  = "bfd plugin starting"
	logLevelDebug                   = "debug"
	logLevelInfo                    = "info"
	logLevelWarn                    = "warn"
	pathEFIConsoleConfig            = "tools/kernel-builder/common/efi-console.config"
	pathGokrazyKernel               = "gokrazy/kernel"
	pathGokrazyKernelConfig         = "gokrazy/kernel/kernel.config"
	pathGokrazyRuntimeConfig        = "gokrazy/kernel/runtime.config"
	pathInstallerKernel             = "tools/installer-kernel"
	pathInstallerKernelConfig       = "tools/installer-kernel/kernel.config"
	pathKernelBuilder               = "tools/kernel-builder"
	pathKernelBuilderCommon         = "tools/kernel-builder/common"
	textRPKIDirectionReceived       = "rpki direction received"
	textUpdateRPKIDirectionReceived = "update-rpki direction received"
	wordWrites                      = "writes"
	wordLint                        = "lint"
)

// Scenario, plugin, profile, mode and case names.
const (
	archAMD64                  = "amd64"
	archARM64                  = "arm64"
	builderDocker              = "docker"
	builderQEMU                = "qemu" // The kernel builder.
	caseNamePipeAliasThief     = "pipe-alias-thief"
	caseNameShapeTypo          = "shape-typo"
	filterNameFirst            = "first" // The name of a declared filter.
	groupDyn                   = "dyn"
	modeAuthPool               = "auth-pool"
	modeBoth                   = "both"
	modeDecode                 = "decode"
	modeStopCCN                = "stopccn"
	nameNoSuchInterface        = "no-such-interface"
	pluginNameAddRemove        = "add-remove"
	profileFast                = "fast"
	profileQEMU                = "qemu" // The kernel profile.
	profileResidential         = "residential"
	scenarioCLIReadsMetaKeys   = "cli-reads-meta-keys"
	scenarioClientBackupStart  = "client-backup-start"
	scenarioClientCachedBoot   = "client-cached-boot"
	scenarioClientFirstBoot    = "client-first-boot"
	scenarioClientReconnect    = "client-reconnect"
	scenarioConfigChangeNotify = "config-change-notify"
	scenarioInitManagedKey     = "init-managed-key"
	sectionPlugins             = "plugins"
	targetInstaller            = "installer"
	targetRuntime              = "runtime"
	versionKernel711           = "7.1.1"
)

// Plain values a fixture compares against.
const (
	valueAlways       = "always"
	valueFalse        = "false"
	valueHundred      = "100"
	valueRedacted     = "<set>"
	valueSecret       = "secret"
	valueTestPassword = "testpass"
	valueTestUser     = "admin"
	valueTrue         = "true"
	valueUnknown      = "unknown"
	valueYes          = "yes"
)
