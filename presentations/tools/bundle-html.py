#!/usr/bin/env python3
"""Bundle an HTML file into a single self-contained file.

Finds <img src="..."> and <link href="..."> references to local files,
base64-encodes them inline as data URIs.

Input must be a .html file. Output is always <name>-inlined.html
in the same directory.

Usage: python3 bundle-html.py <input.html> [<input2.html> ...]
"""

import base64
import os
import re
import sys

MIME = {
    '.png': 'image/png',
    '.jpg': 'image/jpeg',
    '.jpeg': 'image/jpeg',
    '.gif': 'image/gif',
    '.svg': 'image/svg+xml',
    '.webp': 'image/webp',
    '.ico': 'image/x-icon',
    '.css': 'text/css',
    '.js': 'application/javascript',
    '.woff': 'font/woff',
    '.woff2': 'font/woff2',
}


def to_data_uri(base_dir, path):
    full = os.path.normpath(os.path.join(base_dir, path))
    if not os.path.isfile(full):
        return None
    ext = os.path.splitext(full)[1].lower()
    mime = MIME.get(ext)
    if mime is None:
        return None
    with open(full, 'rb') as f:
        data = base64.b64encode(f.read()).decode('ascii')
    return f'data:{mime};base64,{data}'


def replace_or_insert_meta(html, name, content):
    escaped = content.replace('&', '&amp;').replace('"', '&quot;')
    pattern = re.compile(r'(<meta\s+name="' + re.escape(name) + r'"\s+content=")([^"]*)(">)')
    if pattern.search(html):
        return pattern.sub(r'\1' + escaped + r'\3', html, count=1)
    return html.replace('<title>', '<meta name="' + name + '" content="' + escaped + '">\n    <title>', 1)


def replace_or_insert_property(html, prop, content):
    escaped = content.replace('&', '&amp;').replace('"', '&quot;')
    pattern = re.compile(r'(<meta\s+property="' + re.escape(prop) + r'"\s+content=")([^"]*)(">)')
    if pattern.search(html):
        return pattern.sub(r'\1' + escaped + r'\3', html, count=1)
    return html.replace('</title>', '</title>\n    <meta property="' + prop + '" content="' + escaped + '">', 1)


def standalone_title(title):
    if 'standalone HTML deck' in title:
        return title
    title = re.sub(r'\s+-\s+[^<]* slides$', '', title)
    return title + ' - standalone HTML deck'


def mark_standalone_deck(html, input_path, output_path):
    input_name = os.path.basename(input_path)
    output_name = os.path.basename(output_path)

    title_match = re.search(r'<title>(.*?)</title>', html, re.S)
    title = standalone_title(title_match.group(1).strip()) if title_match else 'Standalone HTML deck'
    if title_match:
        html = re.sub(r'<title>.*?</title>', '<title>' + title + '</title>', html, count=1, flags=re.S)

    desc_match = re.search(r'<meta\s+name="description"\s+content="([^"]*)">', html)
    if desc_match and not desc_match.group(1).startswith('Single-file downloadable HTML deck'):
        desc = 'Single-file downloadable HTML deck for local offline use: ' + desc_match.group(1)
        html = replace_or_insert_meta(html, 'description', desc)
        html = replace_or_insert_property(html, 'og:description', desc)
    elif desc_match:
        desc = desc_match.group(1)
    else:
        desc = 'Single-file downloadable HTML deck for local offline use.'
        html = replace_or_insert_meta(html, 'description', desc)
        html = replace_or_insert_property(html, 'og:description', desc)

    html = replace_or_insert_property(html, 'og:title', title)
    html = re.sub(
        r'(<link\s+rel="canonical"\s+href="[^"]*)' + re.escape(input_name) + r'("[^>]*>)',
        r'\1' + output_name + r'\2',
        html,
        count=1,
    )
    html = re.sub(
        r'(<meta\s+property="og:url"\s+content="[^"]*)' + re.escape(input_name) + r'("[^>]*>)',
        r'\1' + output_name + r'\2',
        html,
        count=1,
    )
    return html


def bundle(html, base_dir):
    def replace_src(m):
        prefix = m.group(1)
        path = m.group(2)
        suffix = m.group(3)
        if path.startswith(('http://', 'https://', 'data:')):
            return m.group(0)
        uri = to_data_uri(base_dir, path)
        if uri is None:
            print(f'  skip: {path} (not found or unsupported)', file=sys.stderr)
            return m.group(0)
        print(f'  inline: {path}', file=sys.stderr)
        return f'{prefix}{uri}{suffix}'

    html = re.sub(
        r'(<img\s[^>]*src=")([^"]+)(")',
        replace_src, html,
    )
    html = re.sub(
        r'(<img\s[^>]*src=\')([^\']+)(\')',
        replace_src, html,
    )
    html = re.sub(
        r'(<link\s[^>]*href=")([^"]+)(")',
        replace_src, html,
    )

    def replace_iframe(m):
        tag = m.group(0)
        path = m.group(1)
        if path.startswith(('http://', 'https://', 'data:')):
            return tag
        full = os.path.normpath(os.path.join(base_dir, path))
        if not os.path.isfile(full):
            print(f'  skip: {path} (not found)', file=sys.stderr)
            return tag
        with open(full) as f:
            content = f.read()
        escaped = content.replace('&', '&amp;').replace('"', '&quot;')
        print(f'  inline iframe: {path}', file=sys.stderr)
        return tag.replace(f'src="{path}"', f'srcdoc="{escaped}"')

    html = re.sub(
        r'<iframe\s[^>]*src="([^"]+)"[^>]*>',
        replace_iframe, html,
    )

    # Collect data tags to inject before the first <script>
    data_tags = []

    # Embed slides.md as base64
    slides_path = os.path.join(base_dir, 'slides.md')
    if 'id="embedded-slides"' not in html and os.path.isfile(slides_path):
        with open(slides_path, 'rb') as f:
            md_b64 = base64.b64encode(f.read()).decode('ascii')
        data_tags.append(f'<script id="embedded-slides" type="text/plain">{md_b64}</script>')
        print(f'  embed: slides.md', file=sys.stderr)

    # Embed HTML files referenced by <!-- embed: --> in slides.md
    slides_md = os.path.join(base_dir, 'slides.md')
    if os.path.isfile(slides_md):
        with open(slides_md) as f:
            slides_content = f.read()
        for m in re.finditer(r'<!--\s*embed:\s*(.+?)\s*-->', slides_content):
            raw_name = m.group(1)
            embed_name = re.sub(r'[^a-z0-9._-]+', '-', raw_name, flags=re.I).lower()
            embed_file = os.path.join(base_dir, embed_name)
            if os.path.isfile(embed_file):
                with open(embed_file, 'rb') as f:
                    content_b64 = base64.b64encode(f.read()).decode('ascii')
                elem_id = 'embedded-' + embed_name.replace('.', '-')
                data_tags.append(f'<script id="{elem_id}" type="text/plain">{content_b64}</script>')
                print(f'  embed html: {embed_name}', file=sys.stderr)

    # Insert all data tags before the first <script>
    if data_tags:
        first_script = html.find('<script>')
        inject = '\n'.join(data_tags) + '\n'
        if first_script >= 0:
            html = html[:first_script] + inject + html[first_script:]
        else:
            html = html.replace('</body>', inject + '</body>')

    # Embed screenshots as a JS lookup for dynamic src construction
    screenshots_dir = os.path.join(base_dir, 'screenshots')
    if os.path.isdir(screenshots_dir):
        entries = {}
        for name in sorted(os.listdir(screenshots_dir)):
            full = os.path.join(screenshots_dir, name)
            if not os.path.isfile(full):
                continue
            ext = os.path.splitext(name)[1].lower()
            mime = MIME.get(ext)
            if mime is None:
                continue
            with open(full, 'rb') as f:
                data = base64.b64encode(f.read()).decode('ascii')
            entries[name] = f'data:{mime};base64,{data}'
            print(f'  embed screenshot: {name}', file=sys.stderr)
        if entries:
            js_map = 'var _inlinedScreenshots = {\n'
            for k, v in entries.items():
                js_map += f'  "{k}": "{v}",\n'
            js_map += '};\n'
            inject = f'<script>\n{js_map}</script>\n'
            # Insert before the first <script> so the map is available when the main JS runs
            first_script = html.find('<script>')
            if first_script >= 0:
                html = html[:first_script] + inject + html[first_script:]
            else:
                html = html.replace('</body>', inject + '</body>')
            # Patch JS screenshot src construction to use the map
            html = html.replace(
                """'screenshots/' + escapeHtml(imgName) + '.png'""",
                """(_inlinedScreenshots[escapeHtml(imgName) + '.png'] || 'screenshots/' + escapeHtml(imgName) + '.png')""",
            )

    return html


def inlined_path(input_path):
    if not input_path.endswith('.html'):
        print(f'error: {input_path} does not end with .html', file=sys.stderr)
        sys.exit(1)
    if input_path.endswith('-inlined.html'):
        print(f'error: {input_path} is already an inlined file', file=sys.stderr)
        sys.exit(1)
    return input_path[:-5] + '-inlined.html'


def main():
    if len(sys.argv) < 2:
        print(f'usage: {sys.argv[0]} <input.html> [<input2.html> ...]', file=sys.stderr)
        sys.exit(1)

    for input_path in sys.argv[1:]:
        output_path = inlined_path(input_path)
        base_dir = os.path.dirname(os.path.abspath(input_path))

        print(f'{input_path}:', file=sys.stderr)
        with open(input_path) as f:
            html = f.read()

        result = mark_standalone_deck(bundle(html, base_dir), input_path, output_path)

        with open(output_path, 'w') as f:
            f.write(result)
        print(f'  wrote: {output_path}', file=sys.stderr)


if __name__ == '__main__':
    main()
