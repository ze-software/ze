// Design: docs/architecture/core-design.md -- the htmx upgrade gate, as compiled Go
// Detail: scanner.go -- the DOM, inheritance, and text scanner that reads these rules.
//
// These tables transcribe htmx 4.0.0's upgrade checker. Dictionary order
// is observable when several findings share a line, so ordered source tables
// remain slices. TestUpstreamScannerContractDigest guards every value and order.

package htmxupgrade

const (
	upstreamScannerVersion = "4.0.0"
	upstreamScannerURL     = "https://unpkg.com/htmx.org@4.0.0/dist/scripts/upgrade-check.py"
	upstreamSourceSHA256   = "abcf7cc3ce3162911a1352ed7ad21aa32da09c8f9a92725ba32d7e29d3ca480b"
	scannerContractSHA256  = "3e4faee901b9b437d2220b7402689f8f2af4d298dd97a6ff4dacd9c496db9a4b"
)

type renameRule struct {
	old string
	new string
}

var removedAttrs = []renameRule{
	{old: "hx-vars", new: "use hx-vals with js: prefix"},
	{old: "hx-params", new: "use htmx:config:request event"},
	{old: "hx-prompt", new: "load the hx-prompt extension to keep the same syntax"},
	{old: "hx-ext", new: "include extension scripts directly (no attribute needed in v4)"},
	{old: "hx-disinherit", new: "not needed (inheritance is explicit in v4)"},
	{old: "hx-inherit", new: "not needed (inheritance is explicit in v4)"},
	{old: "hx-request", new: "use hx-config"},
	{old: "hx-history", new: "removed (no localStorage cache in v4)"},
}

var renamedAttrs = []renameRule{
	{old: "hx-disable", new: "rename to hx-ignore (hx-disable now means 'disable during request')"},
	{old: "hx-disabled-elt", new: "rename to hx-disable"},
}

// The two inheritable attributes checkInheritedAttrs branches on by name.
// hx-boost reaches every <a> and <form> beneath it. hx-headers is the only
// carrier the scanner reports with no requesting descendant.
const (
	attrBoost   = "hx-boost"
	attrHeaders = "hx-headers"
)

var inheritableAttrs = map[string]bool{
	attrBoost:         true,
	"hx-confirm":      true,
	"hx-encoding":     true,
	attrHeaders:       true,
	"hx-include":      true,
	"hx-indicator":    true,
	"hx-push-url":     true,
	"hx-replace-url":  true,
	"hx-select":       true,
	"hx-select-oob":   true,
	"hx-swap":         true,
	"hx-sync":         true,
	"hx-target":       true,
	"hx-vals":         true,
	"hx-disabled-elt": true,
}

var requestAttrs = map[string]bool{
	"hx-get":    true,
	"hx-post":   true,
	"hx-put":    true,
	"hx-patch":  true,
	"hx-delete": true,
}

// The rename targets that more than one htmx 1.x event maps to. Each is named
// once so the many-to-one mapping in eventRenames is checked by the compiler.
const (
	eventAfterInit = "htmx:after:init"
	eventAfterSwap = "htmx:after:swap"
	eventError     = "htmx:error"
)

var eventRenames = []renameRule{
	{old: "htmx:afterOnLoad", new: eventAfterInit},
	{old: "htmx:afterProcessNode", new: eventAfterInit},
	{old: "htmx:afterRequest", new: "htmx:after:request"},
	{old: "htmx:afterSettle", new: eventAfterSwap},
	{old: "htmx:afterSwap", new: eventAfterSwap},
	{old: "htmx:beforeCleanupElement", new: "htmx:before:cleanup"},
	{old: "htmx:beforeHistorySave", new: "htmx:before:history:update"},
	{old: "htmx:beforeOnLoad", new: "htmx:before:init"},
	{old: "htmx:beforeProcessNode", new: "htmx:before:process"},
	{old: "htmx:beforeRequest", new: "htmx:before:request"},
	{old: "htmx:beforeSwap", new: "htmx:before:swap"},
	{old: "htmx:configRequest", new: "htmx:config:request"},
	{old: "htmx:historyCacheMiss", new: "htmx:before:history:restore"},
	{old: "htmx:historyRestore", new: "htmx:before:history:restore"},
	{old: "htmx:load", new: eventAfterInit},
	{old: "htmx:oobAfterSwap", new: eventAfterSwap},
	{old: "htmx:oobBeforeSwap", new: "htmx:before:swap"},
	{old: "htmx:pushedIntoHistory", new: "htmx:after:history:push"},
	{old: "htmx:replacedInHistory", new: "htmx:after:history:replace"},
	{old: "htmx:responseError", new: "htmx:response:error"},
	{old: "htmx:sendError", new: eventError},
	{old: "htmx:swapError", new: eventError},
	{old: "htmx:targetError", new: eventError},
	{old: "htmx:timeout", new: eventError},
}

var removedEvents = []string{
	"htmx:validation:validate",
	"htmx:validation:failed",
	"htmx:validation:halted",
	"htmx:xhr:loadstart",
	"htmx:xhr:loadend",
	"htmx:xhr:progress",
	"htmx:xhr:abort",
}

var sseEventRenames = []renameRule{
	{old: "htmx:sseOpen", new: "htmx:sse:after:connection"},
	{old: "htmx:sseError", new: "htmx:sse:error"},
	{old: "htmx:sseBeforeMessage", new: "htmx:sse:before:message"},
	{old: "htmx:sseMessage", new: "htmx:sse:after:message"},
	{old: "htmx:sseClose", new: "htmx:sse:close"},
}

// htmx 4.0.0 dispatches one message event for each direction, so both htmx 1.x
// send events map to the outgoing one. Naming it once has the compiler check
// that many-to-one mapping.
const eventWSOutgoingBefore = "htmx:ws:before:message:outgoing"

var wsEventRenames = []renameRule{
	{old: "htmx:wsOpen", new: "htmx:ws:after:connection"},
	{old: "htmx:wsClose", new: "htmx:ws:close"},
	{old: "htmx:wsConfigSend", new: eventWSOutgoingBefore},
	{old: "htmx:wsBeforeSend", new: eventWSOutgoingBefore},
	{old: "htmx:wsAfterSend", new: "htmx:ws:after:message:outgoing"},
	{old: "htmx:wsBeforeMessage", new: "htmx:ws:before:message:incoming"},
	{old: "htmx:wsAfterMessage", new: "htmx:ws:after:message:incoming"},
}

var extensionAttrRenames = []renameRule{
	{old: "sse-connect", new: "rename to hx-sse:connect"},
	{old: "sse-close", new: "rename to hx-sse:close"},
	{old: "sse-swap", new: "removed — SSE now integrates with standard htmx request pipeline"},
	{old: "ws-connect", new: "rename to hx-ws:connect"},
	{old: "ws-send", new: "rename to hx-ws:send"},
}

var removedJSAPI = []renameRule{
	{old: "htmx.addClass", new: "use element.classList.add()"},
	{old: "htmx.removeClass", new: "use element.classList.remove()"},
	{old: "htmx.toggleClass", new: "use element.classList.toggle()"},
	{old: "htmx.closest", new: "use element.closest()"},
	{old: "htmx.remove", new: "use element.remove()"},
	{old: "htmx.off", new: "use removeEventListener() (htmx.on() returns the callback)"},
	{old: "htmx.location", new: "use htmx.ajax()"},
	{old: "htmx.takeClass", new: "removed from core. To restore the htmx 2 behavior verbatim, paste:\n    htmx.takeClass = (elt, cls) => {\n        elt = typeof elt === 'string' ? document.querySelector(elt) : elt;\n        for (let c of elt.parentElement.children) c.classList.remove(cls);\n        elt.classList.add(cls);\n    };\n  For a fully featured replacement (polymorphic targets/sources, selector strings, iterable inputs, q-proxy chaining), load the hx-live extension and use htmx.live.take()."},
	{old: "htmx.logAll", new: "use htmx.config.logAll = true"},
	{old: "htmx.logNone", new: "use htmx.config.logAll = false (errors/warnings flow to console.* directly)"},
	{old: "htmx.logger", new: "removed; htmx logs to console.error / console.warn / console.log directly. Observability tools (Sentry, DataDog RUM, etc.) capture console.* automatically"},
	{old: "htmx.defineExtension", new: "use htmx.registerExtension()"},
}

var configRenames = []renameRule{
	{old: "defaultSwapStyle", new: "defaultSwap"},
	{old: "globalViewTransitions", new: "transitions"},
	{old: "historyEnabled", new: "history"},
	{old: "includeIndicatorStyles", new: "includeIndicatorCSS"},
}

var removedConfig = []string{
	"addedClass",
	"allowEval",
	"allowNestedOobSwaps",
	"allowScriptTags",
	"attributesToSettle",
	"defaultSwapDelay",
	"disableSelector",
	"getCacheBusterParam",
	"historyCacheSize",
	"methodsThatUseUrlParams",
	"refreshOnHistoryMiss",
	"responseHandling",
	"scrollBehavior",
	"scrollIntoViewOnBoost",
	"selfRequestsOnly",
	"settlingClass",
	"swappingClass",
	"triggerSpecsCache",
	"useTemplateFragments",
	"withCredentials",
	"wsBinaryType",
	"wsReconnectDelay",
}

var removedResponseHeaders = []renameRule{
	{old: "HX-Trigger-After-Swap", new: "use HX-Trigger or JavaScript instead"},
	{old: "HX-Trigger-After-Settle", new: "use HX-Trigger or JavaScript instead"},
}

var defaultExtensions = []string{
	".html", ".php", ".js", ".ts", ".jinja", ".jinja2", ".j2", ".erb", ".hbs",
}

var extraExtensions = []string{".templ", ".go"}

var jsExtensions = map[string]bool{
	".js": true, ".mjs": true, ".cjs": true, ".ts": true,
	".mts": true, ".cts": true, ".jsx": true, ".tsx": true,
}
