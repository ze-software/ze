#!/usr/bin/env python3
"""Validation helpers for website JSON models.

Renderers use these functions before they turn curated JSON into HTML.  The
checks are intentionally small and local: they catch broken fields and unsafe
markup at the point where a content edit would otherwise become malformed
published HTML.
"""

import re


CATEGORIES = {
    "operate",
    "routing",
    "services",
    "automate",
    "observe",
    "secure",
    "platform",
    "meta",
}

FEATURE_STATUSES = {None, "experimental", "aspiration"}

REL_URL_RE = re.compile(r"^(?:[A-Za-z0-9_.@/+:-]+)(?:#[A-Za-z0-9_.@:-]+)?/?$")
ABS_URL_RE = re.compile(r"^https?://[^\s<>\"]+$")
MARKUP_RE = re.compile(r"(<[^>]+>|&(?!(?:amp|lt|gt|quot|#39);))")


class ModelError(ValueError):
    """Raised when a site data file has an invalid shape."""


def _ctx(path, message):
    return "%s: %s" % (path, message)


def require_mapping(value, path):
    if not isinstance(value, dict):
        raise ModelError(_ctx(path, "expected object"))
    return value


def require_list(value, path):
    if not isinstance(value, list):
        raise ModelError(_ctx(path, "expected list"))
    return value


def require_text(value, path, *, allow_empty=False, allow_inline_markup=False):
    if not isinstance(value, str):
        raise ModelError(_ctx(path, "expected string"))
    if not allow_empty and not value.strip():
        raise ModelError(_ctx(path, "must not be empty"))
    if not allow_inline_markup and MARKUP_RE.search(value):
        raise ModelError(_ctx(path, "raw HTML or unsafe entity is not allowed"))
    return value


def require_bool(value, path):
    if not isinstance(value, bool):
        raise ModelError(_ctx(path, "expected boolean"))
    return value


def require_url(value, path, *, external=False):
    text = require_text(value, path)
    pattern = ABS_URL_RE if external else REL_URL_RE
    if not pattern.match(text):
        kind = "absolute http(s) URL" if external else "relative site URL"
        raise ModelError(_ctx(path, "expected %s, got %r" % (kind, text)))
    return text


def validate_nav(data):
    data = require_mapping(data, "nav")
    require_list(data.get("top_links", []), "nav.top_links")
    for dropdown_index, dropdown in enumerate(require_list(data.get("dropdowns"), "nav.dropdowns")):
        dropdown = require_mapping(dropdown, "nav.dropdowns[%d]" % dropdown_index)
        require_text(dropdown.get("label"), "nav.dropdowns[%d].label" % dropdown_index)
        for col_index, column in enumerate(require_list(dropdown.get("columns"), "nav.dropdowns[%d].columns" % dropdown_index)):
            for entry_index, entry in enumerate(require_list(column, "nav.dropdowns[%d].columns[%d]" % (dropdown_index, col_index))):
                path = "nav.dropdowns[%d].columns[%d][%d]" % (dropdown_index, col_index, entry_index)
                entry = require_mapping(entry, path)
                if "label_only" in entry:
                    require_text(entry["label_only"], path + ".label_only")
                    continue
                require_url(entry.get("href"), path + ".href")
                require_text(entry.get("icon"), path + ".icon", allow_inline_markup=True)
                require_text(entry.get("title"), path + ".title")
                require_text(entry.get("desc"), path + ".desc")
                if "feature" in entry:
                    require_bool(entry["feature"], path + ".feature")
    for index, link in enumerate(require_list(data.get("trailing_links", []), "nav.trailing_links")):
        path = "nav.trailing_links[%d]" % index
        link = require_mapping(link, path)
        require_url(link.get("href"), path + ".href")
        require_text(link.get("label"), path + ".label")
    return data


def validate_audience(data):
    data = require_mapping(data, "audience")
    for group_name in ("run", "who"):
        for index, card in enumerate(require_list(data.get(group_name), "audience.%s" % group_name)):
            path = "audience.%s[%d]" % (group_name, index)
            card = require_mapping(card, path)
            require_text(card.get("title"), path + ".title")
            require_text(card.get("label"), path + ".label")
            category = require_text(card.get("category"), path + ".category")
            if category not in CATEGORIES:
                raise ModelError(_ctx(path + ".category", "unknown category %r" % category))
            for chip_index, chip in enumerate(require_list(card.get("chips"), path + ".chips")):
                require_text(chip, path + ".chips[%d]" % chip_index)
            require_text(card.get("body"), path + ".body")
            if "link" in card:
                link = require_mapping(card["link"], path + ".link")
                require_url(link.get("href"), path + ".link.href")
                require_text(link.get("label"), path + ".link.label")
                require_text(link.get("sublabel"), path + ".link.sublabel")
    return data


def validate_whats_new(data):
    """The homepage "what's new" band. Only the freeform note is curated: the
    article and weekly-update slots are generated from the post sources."""
    data = require_mapping(data, "whats-new")
    require_text(data.get("title"), "whats-new.title")
    link = require_mapping(data.get("link"), "whats-new.link")
    require_url(link.get("href"), "whats-new.link.href")
    require_text(link.get("label"), "whats-new.link.label")
    note = data.get("note")
    if note is not None:
        note = require_mapping(note, "whats-new.note")
        require_text(note.get("label"), "whats-new.note.label")
        category = require_text(note.get("category"), "whats-new.note.category")
        if category not in CATEGORIES:
            raise ModelError(
                _ctx("whats-new.note.category", "unknown category %r" % category)
            )
        require_text(note.get("title"), "whats-new.note.title")
        require_text(note.get("body"), "whats-new.note.body")
        if "link" in note:
            note_link = require_mapping(note["link"], "whats-new.note.link")
            require_url(note_link.get("href"), "whats-new.note.link.href")
            require_text(note_link.get("label"), "whats-new.note.link.label")
    return data


def validate_features(data):
    data = require_mapping(data, "features")
    seen_ids = set()
    for section_index, section in enumerate(require_list(data.get("sections"), "features.sections")):
        path = "features.sections[%d]" % section_index
        section = require_mapping(section, path)
        section_id = require_text(section.get("id"), path + ".id")
        if section_id in seen_ids:
            raise ModelError(_ctx(path + ".id", "duplicate section id %r" % section_id))
        seen_ids.add(section_id)
        require_text(section.get("heading"), path + ".heading")
        require_text(section.get("lead"), path + ".lead")
        note = section.get("note")
        if note is not None:
            require_text(note, path + ".note")
        for card_index, card in enumerate(require_list(section.get("cards"), path + ".cards")):
            card_path = path + ".cards[%d]" % card_index
            card = require_mapping(card, card_path)
            category = require_text(card.get("category"), card_path + ".category")
            if category not in CATEGORIES:
                raise ModelError(_ctx(card_path + ".category", "unknown category %r" % category))
            status = card.get("status")
            if status not in FEATURE_STATUSES:
                raise ModelError(_ctx(card_path + ".status", "unknown status %r" % status))
            require_text(card.get("title"), card_path + ".title")
            external = require_bool(card.get("external"), card_path + ".external")
            require_url(card.get("href"), card_path + ".href", external=external)
            for chip_index, chip in enumerate(require_list(card.get("chips"), card_path + ".chips")):
                chip_path = card_path + ".chips[%d]" % chip_index
                chip = require_mapping(chip, chip_path)
                require_text(chip.get("text"), chip_path + ".text")
                require_bool(chip.get("mode"), chip_path + ".mode")
            for bullet_index, bullet in enumerate(require_list(card.get("bullets"), card_path + ".bullets")):
                require_text(
                    bullet,
                    card_path + ".bullets[%d]" % bullet_index,
                    allow_inline_markup=True,
                )
    return data
