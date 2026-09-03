// Design: docs/architecture/testing/ci-format.md -- the compiled fixture API
// Detail: one constant for each string literal this package repeats, named for
// the value it carries rather than for its Go type.
// Detail: a literal that means two unrelated things here gets one constant for
// each meaning, so the text repeats and the names do not. Each such constant
// states its meaning on its own line.
// Related: fixture.go holds the shared fixture helpers.

package fixture

// Environment variables and build tags.
const (
	buildTagLE          = "ze_le"
	envCGOEnabled       = "CGO_ENABLED"
	envCLIFormat        = "ze.cli.format"
	envConfigDir        = "ZE_CONFIG_DIR"
	envLogBFD           = "ze.log.bfd"
	envLogBGP           = "ze.log.bgp"
	envLogVPP           = "ze.log.vpp"
	envNoColor          = "NO_COLOR"
	envPath             = "PATH"
	envReadyFile        = "ZE_READY_FILE"
	envRepoRoot         = "ZE_REPO_ROOT"
	envSSHEphemeral     = "ZE_SSH_EPHEMERAL"
	envSSHHost          = "ZE_SSH_HOST"
	envSSHPassword      = "ZE_SSH_PASSWORD"
	envSSHPort          = "ZE_SSH_PORT"
	envSSHUsername      = "ZE_SSH_USERNAME"
	envTerm             = "TERM"
	envTestBGPPort      = "ze_test_bgp_port"
	envTestBudgetDotted = "ze.test.budget"
	envTestBudgetLower  = "ze_test_budget"
	envTestBudgetUpper  = "ZE_TEST_BUDGET"
)

// Addresses, prefixes, ports and other network identities.
const (
	addrLoopback               = "127.0.0.1"
	addrLoopbackSecond         = "127.0.0.2"
	addrLoopbackThird          = "127.0.0.3"
	addrPeerOne                = "10.0.0.1"
	addrPeerTwo                = "10.0.0.2"
	addrTestNet1First          = "192.0.2.1"
	addrTestNet1Second         = "192.0.2.2"
	addrTestNet2Nexthop        = "198.51.100.162"
	addrTestNet3Nine           = "203.0.113.9"
	asnLabel64501              = "AS64501"
	asnLabel64502              = "AS64502"
	configPathRouterID         = "bgp.router-id"
	familyIPv4Unicast          = "ipv4/unicast"
	hostnamePyPeer             = "py-peer"
	metricPeerStateTransitions = "ze_peer_state_transitions_total"
	metricPoolUsedRatio        = "ze_bgp_pool_used_ratio"
	peerNameOne                = "peer1"
	portL2TP                   = "1701"
	prefixTenOne               = "10.1.0.0/24"
	prefixTenTwenty            = "10.20.0.0/24"
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
	eventState              = "state"  // The event kind a plugin subscribes to.
	eventUpdate             = "update" // The event kind a plugin subscribes to.
	fieldASPath             = "as-path"
	fieldAddress            = "address" // The peer address field in a payload.
	fieldChecks             = "checks"
	fieldCommand            = "command" // The field in a payload or a query.
	fieldCommands           = "commands"
	fieldCommunity          = "community"
	fieldContainer          = "container"
	fieldContent            = "content"
	fieldCount              = "count" // The field in a payload.
	fieldDetailLog          = "detail-log"
	fieldEntries            = "entries"
	fieldError              = "error" // The field that carries the message in a payload.
	fieldErrors             = "errors"
	fieldFamily             = "family"
	fieldFile               = "file"
	fieldFiles              = "files"
	fieldGroupID            = "group-id"
	fieldImage              = "image"
	fieldInput              = "input" // The field in a payload.
	fieldKind               = "kind"
	fieldLeaf               = "leaf"
	fieldLocalAS            = "local-as"
	fieldMessage            = "message"
	fieldMode               = "mode"
	fieldName               = "name" // The field in a payload.
	fieldPackages           = "packages"
	fieldParallel           = "parallel"
	fieldPassed             = "passed"
	fieldPath               = "path"
	fieldPeer               = "peer"  // The field in a payload.
	fieldPeers              = "peers" // The field in a payload.
	fieldPeersConfigured    = "peers-configured"
	fieldPeersEstablished   = "peers-established"
	fieldReady              = "ready"
	fieldRelated            = "related"
	fieldRerun              = "rerun"
	fieldRouterID           = "router-id"
	fieldRows               = "rows"
	fieldStage              = "stage"
	fieldState              = "state" // The peer or interface field in a payload.
	fieldStatus             = "status"
	fieldSummary            = "summary" // The field in a payload.
	fieldTop                = "top"     // The field in a payload.
	fieldTotals             = "totals"
	fieldTunnelID           = "tunnel-id"
	fieldType               = "type"    // The field in a payload.
	fieldUpdated            = "updated" // The field in a payload.
	fieldVRPCount           = "vrp-count"
	fieldValue              = "value"
	fieldWritten            = "written"
	linkStateDown           = "down"
	messageTypeUpdateRPKI   = "update-rpki"
	namespaceBGP            = "bgp"
	outcomeFailed           = "failed"
	outcomeSuccess          = "success"
	severityError           = "error" // The severity of a diagnostic entry.
	severityWarning         = "warning"
	sourceCommand           = "command" // The origin recorded for a config node.
	sourceConfig            = "config"  // The origin recorded for a config node.
	sourceGroup             = "group"   // The origin recorded for a config node.
	sourcePeer              = "peer"    // The origin recorded for a config node.
	stateEstablished        = "established"
	stateNotSynchronized    = "not-synchronized"
	statusCommitted         = "committed"
	statusDone              = "done"
	statusError             = "error" // The status a dispatch returns.
	statusMonitorConfigured = "monitor-configured"
	statusStale             = "stale"
	statusUpdated           = "updated" // The status a config write returns.
	verdictInvalid          = "invalid"
	verdictPass             = "pass"
	verdictValid            = "valid"
)

// Table columns, render operators, pipe operators and answer shapes.
const (
	aliasPeers               = "peers"   // The name of a declared pipe alias.
	aliasSummary             = "summary" // The name of a declared pipe alias.
	columnAddress            = "address" // The column in a rendered table.
	columnConnectionsDropped = "connections-dropped"
	columnGroup              = "group" // The column in a rendered table.
	columnName               = "name"  // The column in a rendered table.
	columnPrefix             = "prefix"
	columnRemoteAS           = "remote-as"
	columnState              = "state" // The column in a rendered table.
	columnUptime             = "uptime"
	headerContentType        = "Content-Type"
	mediaTypeJSON            = "application/json"
	pipeCount                = "count" // The pipe operator.
	pipeFirst                = "first" // The pipe operator.
	pipeOrigin               = "origin"
	pipeResolve              = "resolve"
	renderJSON               = "json"
	renderRaw                = "raw"
	renderTable              = "table" // The pipe operator and the answer shape.
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
	argConfig                    = "config"  // The git subcommand.
	argInit                      = "init"
	argInterface                 = "interface"
	argName                      = "name" // The command keyword.
	argShow                      = "show"
	argType                      = "type" // The command keyword.
	cmdAnnounceFirstPrefix       = "update text nhop 101.1.101.1 nlri ipv4/unicast add 1.1.0.0/24"
	cmdShowBGP                   = "show bgp"
	cmdShowBGPPeerList           = "show bgp peer list"
	cmdShowNsaliasCounters       = "show nsalias counters"
	cmdShowPipealiasCounters     = "show pipealias counters"
	cmdShowPipehelpCounters      = "show pipehelp counters"
	cmdShowTestCompletionHidden  = "show test-completion hidden"
	cmdShowTestCompletionVisible = "show test-completion visible"
	cmdShowTestTwoTextsBoth      = "show test-twotexts both"
	cmdShowTestTwoTextsSummary   = "show test-twotexts summary"
	cmdShowVersion               = "show version"
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
	programSSH                   = "ssh"
	xfrmWordSource               = "src" // The ip xfrm output column.
)

// le areas, actions, checks and gates.
const (
	actionAccept                  = "accept" // The policy decision recorded for a route.
	actionCheck                   = "check"
	actionCondensedCheck          = "condensed-check"
	actionIndexCheck              = "index-check"
	actionIndexUpdate             = "index-update"
	actionL2TPTest                = "l2tp-test"
	actionPayloadReport           = "payload-report"
	actionReject                  = "reject"
	actionReleaseCandidate        = "release-candidate"
	actionReport                  = "report"
	actionRouterReport            = "router-report"
	actionRun                     = "run"
	actionSelftest                = "selftest"
	actionStart                   = "start"
	actionTreeCheck               = "tree-check"
	actionUpdate                  = "update" // The le action verb.
	actionVPPHugepagesTest        = "vpp-hugepages-test"
	actionVPPIfaceTest            = "vpp-iface-test"
	actionVPPTest                 = "vpp-test"
	areaCLI                       = "cli"
	areaDeployment                = "deployment"
	areaDigest                    = "digest"
	areaEvidence                  = "evidence"
	areaQEMU                      = "qemu" // The le area.
	areaRepository                = "repository"
	areaRules                     = "rules"
	areaTier                      = "tier"
	checkCIDispatch               = "ci-dispatch"
	checkCLIGrammar               = "cli-grammar"
	checkCommandOwnership         = "command ownership"
	checkConfigClaims             = "config claims"
	checkConfigCoercion           = "config coercion"
	checkDashStdio                = "dash-stdio"
	checkDocWiring                = "doc wiring"
	checkFSPersistence            = "fs-persistence"
	checkIfaceResolution          = "iface-resolution"
	checkPluginBoundary           = "plugin boundary"
	checkPortDefaults             = "port-defaults"
	checkRepositoryTrackedBuild   = "repository tracked-build"
	checkStaticcheckFeatureMatrix = "staticcheck-feature-matrix"
	checkTestSensitivity          = "test-sensitivity"
	checkYANGLeafMentions         = "yang leaf-mentions"
	gateGokrazyGosum              = "gokrazy-gosum"
	gateProtocolSkeleton          = "protocol-skeleton"
)

// Files, paths and expected text.
const (
	contentFeatureGate              = "fixture\n"
	descriptionCountersAlone        = "The counters alone"
	dirBuilder                      = "builder"
	dirCommon                       = "common"
	dirSource                       = "src" // The kernel source directory.
	expansionVRPCount               = "display kind vrp-count"
	fileBGPConf                     = "ze-bgp.conf"
	fileConfig2Conf                 = "config2.conf"
	fileCoreMD                      = "CORE.md"
	fileDaemonPID                   = "daemon.pid"
	fileDaemonReady                 = "daemon.ready"
	fileFeatureGates                = "feature-gates.txt"
	fileGoMod                       = "go.mod"
	fileSampleGo                    = "sample.go"
	fileTriggersMD                  = "TRIGGERS.md"
	logBFDConfigured                = "bfd plugin configured"
	logBFDRunning                   = "bfd plugin running"
	logBFDStarting                  = "bfd plugin starting"
	logLevelDebug                   = "debug"
	logLevelInfo                    = "info"
	logLevelWarn                    = "warn"
	pathDocWiringLog                = "tmp/verify/doc-wiring.log"
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
	groupRoute                 = "route" // The group a watched object belongs to.
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
	valueTrue         = "true"
	valueUnknown      = "unknown"
	valueYes          = "yes"
)
