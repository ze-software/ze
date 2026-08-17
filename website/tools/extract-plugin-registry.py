#!/usr/bin/env python3
"""Parse ../main/internal/**/register.go for plugin registration data.

Usage:
    tools/extract-plugin-registry.py

This is a plain-text/regex parser over Go source, not a Go AST tool and not
an LLM-curated mapping: every field in data/plugin-registry.json is a
mechanical extraction from a real `registry.Registration{...}` composite
literal (internal/component/plugin/registry/registry.go's Registration
struct: Name, Description, ConfigRoots, Dependencies,
OptionalDependencies). Field values that are bare identifiers rather than
string literals (e.g. `Name: pluginName`) are resolved by scanning every
other .go file in the same package directory for a matching `const`/`var`
string declaration -- if no such declaration is found, the value is kept
as "?identifier" rather than silently dropped or guessed.

This is the grouping data source for tools/render-config-reference.py and
tools/render-plugin-catalog.py. Plugin catalog prose/formatting can be
extended with local `PLUGIN.md` front matter next to a plugin's `register.go`;
the extractor keeps that metadata distributed with the plugin instead of in a
central website list.
"""

import json
import pathlib
import re
import sys

import sitelib
import sitepaths

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
MAIN_REPO = sitepaths.MAIN_REPO
INTERNAL_DIR = MAIN_REPO / "internal"
DEST = GH_PAGES / "data" / "plugin-registry.json"

MODULE_PREFIX = "github.com/ze-software/ze/"

STRING_LIT = r'"(?:\\.|[^"\\])*"'
IDENT = r"[A-Za-z_][A-Za-z0-9_]*"
QUALIFIED_IDENT = IDENT + r"(?:\." + IDENT + r")?"

FIELD_RE = re.compile(
    r"\b(Name|Description|ConfigRoots|Dependencies|OptionalDependencies|YANG)\s*:\s*"
    r"(\[\]string\s*\{[^}]*\}|" + STRING_LIT + r"|" + QUALIFIED_IDENT + r")",
    re.DOTALL,
)
LIST_ITEM_RE = re.compile(STRING_LIT + r"|" + IDENT)
CONST_SINGLE_RE = re.compile(
    r"\b(?:const|var)\s+(" + IDENT + r")\s+(?:string\s+)?=\s*(" + STRING_LIT + r")"
)
CONST_BLOCK_RE = re.compile(r"\b(?:const|var)\s*\(([^)]*)\)", re.DOTALL)
CONST_BLOCK_LINE_RE = re.compile(
    r"\b(" + IDENT + r")\s+(?:string\s+)?=\s*(" + STRING_LIT + r")"
)
IMPORT_BLOCK_RE = re.compile(r"\bimport\s*\(([^)]*)\)", re.DOTALL)
IMPORT_LINE_RE = re.compile(r"(?:(" + IDENT + r")\s+)?\"([^\"]+)\"")
FRONT_MATTER_RE = re.compile(r"^---\n(.*?)\n---\n?(.*)$", re.DOTALL)
PLUGIN_DOC_NAMES = ("PLUGIN.md", "plugin.md")


def unquote(s):
    if s.startswith('"') and s.endswith('"'):
        return s[1:-1].encode().decode("unicode_escape")
    return s


def find_registration_blocks(text):
    """Yield the text of every `registry.Registration{...}` composite
    literal in a file, using brace-depth counting (not a single regex,
    since these literals nest other {}s -- ConfigRoots: []string{...},
    closures, etc.)."""
    for m in re.finditer(r"registry\.Registration\s*\{", text):
        start = m.end() - 1
        depth = 0
        i = start
        while i < len(text):
            if text[i] == "{":
                depth += 1
            elif text[i] == "}":
                depth -= 1
                if depth == 0:
                    yield text[start : i + 1]
                    break
            i += 1


def strip_go_comments(text):
    """Remove // line and /* */ block comments, copying string, rune, and
    raw literals verbatim so a `//` or `)` inside a string survives. Needed
    because a `)` in a comment inside a `const ( ... )` block would otherwise
    truncate CONST_BLOCK_RE's non-paren match before it reaches the const."""
    out = []
    i, n = 0, len(text)
    while i < n:
        c = text[i]
        if c in "\"'`":
            out.append(c)
            i += 1
            while i < n:
                ch = text[i]
                out.append(ch)
                if ch == "\\" and c != "`":  # escape inside "" or '' literals
                    if i + 1 < n:
                        out.append(text[i + 1])
                        i += 2
                        continue
                i += 1
                if ch == c:
                    break
            continue
        if c == "/" and i + 1 < n and text[i + 1] == "/":
            while i < n and text[i] != "\n":
                i += 1
            continue
        if c == "/" and i + 1 < n and text[i + 1] == "*":
            i += 2
            while i + 1 < n and not (text[i] == "*" and text[i + 1] == "/"):
                i += 1
            i += 2
            continue
        out.append(c)
        i += 1
    return "".join(out)


def build_symbol_table(package_dir):
    """const/var NAME = "literal" declarations across every non-test .go
    file in a plugin's package directory (single-line and grouped const()
    block forms) -- resolves bare-identifier field values like
    `Name: pluginName` back to their real string."""
    symbols = {}
    for go_file in package_dir.glob("*.go"):
        if go_file.name.endswith("_test.go"):
            continue
        text = strip_go_comments(go_file.read_text(errors="replace"))
        for name, value in CONST_SINGLE_RE.findall(text):
            symbols.setdefault(name, unquote(value))
        for block in CONST_BLOCK_RE.findall(text):
            for name, value in CONST_BLOCK_LINE_RE.findall(block):
                symbols.setdefault(name, unquote(value))
    return symbols


def import_alias_map(text):
    """package-alias -> import path, e.g. {"bgpyang": "github.com/.../bgp/yang"}.
    Only handles explicitly-aliased imports (alias "path"); this codebase's
    own convention is to always alias a yang sub-package import (e.g.
    `bgpyang "…/bgp/yang"`, `dhcpyang "…/dhcpserver/yang"`), so a bare
    (unaliased) import is never the YANG field's source package here."""
    aliases = {}
    for block in IMPORT_BLOCK_RE.findall(text):
        for alias, path in IMPORT_LINE_RE.findall(block):
            if alias:
                aliases[alias] = path
    return aliases


def yang_dir_from_field(yang_value, aliases):
    """Resolve a `YANG: pkgalias.ConstName` field to the real directory its
    package lives in, via the file's own import statement -- not an
    assumed `<source_dir>/yang/` convention, since at least one real plugin
    (the core "bgp" registration) imports its YANG from a sibling
    directory rather than its own."""
    if not yang_value or "." not in yang_value:
        return None
    alias = yang_value.split(".", 1)[0]
    import_path = aliases.get(alias)
    if not import_path or not import_path.startswith(MODULE_PREFIX):
        return None
    return MAIN_REPO / import_path[len(MODULE_PREFIX) :]


def resolve_token(token, symbols):
    token = token.strip()
    if not token:
        return None
    if token.startswith('"'):
        return unquote(token)
    return symbols.get(token, "?%s" % token)


def resolve_list(raw, symbols):
    inner = raw[raw.index("{") + 1 : raw.rindex("}")]
    out = []
    for item in LIST_ITEM_RE.findall(inner):
        resolved = resolve_token(item, symbols)
        if resolved is not None:
            out.append(resolved)
    return out


def parse_front_matter(text):
    m = FRONT_MATTER_RE.match(text)
    if not m:
        return {}, text.strip()
    raw, body = m.group(1), m.group(2)
    meta = {}
    for line in raw.splitlines():
        if ":" not in line:
            continue
        key, _, value = line.partition(":")
        meta[key.strip().lower()] = value.strip()
    return meta, body.strip()


def split_csv(value):
    return [item.strip() for item in value.split(",") if item.strip()]


def load_plugin_doc(package_dir):
    """Optional local catalog metadata kept beside the plugin implementation.

    The file is deliberately per-plugin front matter, not a central taxonomy.
    Supported keys: title, summary, area or catalog-area, tags.
    """
    for name in PLUGIN_DOC_NAMES:
        path = package_dir / name
        if not path.exists():
            continue
        meta, body = parse_front_matter(path.read_text(errors="replace"))
        doc = {"source": str(path.relative_to(MAIN_REPO))}
        if meta.get("title"):
            doc["title"] = meta["title"]
        if meta.get("summary"):
            doc["summary"] = meta["summary"]
        area = meta.get("area") or meta.get("catalog-area")
        if area:
            doc["area"] = area
        tags = split_csv(meta.get("tags", ""))
        if tags:
            doc["tags"] = tags
        if body:
            doc["body"] = body
        return doc
    return {}


def parse_register_file(path):
    text = path.read_text(errors="replace")
    if "registry.Registration{" not in text:
        return []
    symbols = build_symbol_table(path.parent)
    aliases = import_alias_map(text)
    entries = []
    for block in find_registration_blocks(text):
        fields = {
            "name": None,
            "description": None,
            "config_roots": [],
            "dependencies": [],
            "optional_dependencies": [],
        }
        yang_field = None
        for key, value in FIELD_RE.findall(block):
            if key == "Name":
                fields["name"] = resolve_token(value, symbols)
            elif key == "Description":
                fields["description"] = resolve_token(value, symbols)
            elif key == "ConfigRoots":
                fields["config_roots"] = resolve_list(value, symbols)
            elif key == "Dependencies":
                fields["dependencies"] = resolve_list(value, symbols)
            elif key == "OptionalDependencies":
                fields["optional_dependencies"] = resolve_list(value, symbols)
            elif key == "YANG":
                yang_field = value.strip()
        if not fields["name"]:
            continue

        # Resolve the YANG source directory via the actual import statement
        # for the field's package alias, falling back to the by-convention
        # <source_dir>/yang/ only if the field is absent or unresolvable.
        yang_dir = yang_dir_from_field(yang_field, aliases) if yang_field else None
        if yang_dir is None or not yang_dir.is_dir():
            fallback = path.parent / "yang"
            yang_dir = fallback if fallback.is_dir() else None
        yang_files = (
            sorted(str(p.relative_to(MAIN_REPO)) for p in yang_dir.glob("*.yang"))
            if yang_dir is not None
            else []
        )
        fields["source_dir"] = str(path.parent.relative_to(MAIN_REPO))
        fields["yang_files"] = yang_files
        fields["doc"] = load_plugin_doc(path.parent)
        entries.append(fields)
    return entries


def main():
    if not INTERNAL_DIR.is_dir():
        print("error: %s not found" % INTERNAL_DIR, file=sys.stderr)
        return 1

    plugins = []
    for register_go in sorted(INTERNAL_DIR.rglob("register.go")):
        if ".claude" in register_go.parts:
            continue
        plugins.extend(parse_register_file(register_go))

    unresolved = [
        (p["name"], field, val)
        for p in plugins
        for field in ("config_roots", "dependencies", "optional_dependencies")
        for val in p[field]
        if isinstance(val, str) and val.startswith("?")
    ]
    for name, field, val in unresolved:
        sitelib.warn(
            "%s: could not resolve identifier %r in %s "
            "(no matching const/var found in package dir)" % (name, val[1:], field)
        )

    DEST.parent.mkdir(parents=True, exist_ok=True)
    DEST.write_text(json.dumps(plugins, indent=2, ensure_ascii=False) + "\n")
    print("extracted %d plugin registrations -> %s" % (len(plugins), DEST))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
