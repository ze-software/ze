#!/usr/bin/env -S uv run --with markdown python3

import importlib.util
import pathlib
import sys
import unittest


HERE = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
SPEC = importlib.util.spec_from_file_location("render_blog", HERE / "render-blog.py")
render_blog = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(render_blog)


def article(**updates):
    value = {
        "slug": "example",
        "title": "Example article",
        "date": "2026-08-22",
        "author": "Thomas Mangin",
        "description": "Example description.",
        "deck": "Example deck.",
        "image": "",
        "image_alt": "",
        "image_dark": "",
        "key_points": [],
        "body": "## First section\n\nBody.\n\n## Second section\n\nBody.\n\n## Third section\n\nBody.",
    }
    value.update(updates)
    return value


class ArticleHeroTest(unittest.TestCase):
    def test_image_controls_two_column_hero(self):
        without_image = render_blog.render_article(article())
        with_image = render_blog.render_article(
            article(image="assets/blog/example.svg", image_alt="Example diagram.")
        )

        self.assertIn('class="blog-article-shell"', without_image)
        self.assertNotIn('class="blog-article-shell has-visual"', without_image)
        self.assertNotIn('class="blog-article-visual', without_image)
        self.assertIn('class="blog-article-shell has-visual"', with_image)
        self.assertIn('class="blog-article-visual', with_image)

    def test_metadata_is_consistent_with_or_without_deck(self):
        with_deck = render_blog.render_article(article())
        without_deck = render_blog.render_article(article(deck=""))

        for rendered in (with_deck, without_deck):
            self.assertIn('class="blog-article-meta"', rendered)
            self.assertIn('datetime="2026-08-22"', rendered)
            self.assertIn("by Thomas Mangin", rendered)

        self.assertIn("Example deck.", with_deck)
        self.assertIn("Example description.", without_deck)

    def test_article_lead_is_escaped(self):
        rendered = render_blog.render_article(
            article(deck="BGP < OSPF & routing")
        )

        self.assertIn("BGP &lt; OSPF &amp; routing", rendered)
        self.assertNotIn("BGP < OSPF", rendered)

    def test_index_marks_image_cards_for_media_layout(self):
        rendered = render_blog.render_index(
            [article(image="assets/blog/article.svg", image_alt="Article diagram.")]
        )

        self.assertIn('class="card card-post blog-card has-media ', rendered)
        self.assertIn('src="../assets/blog/article.svg"', rendered)
        self.assertIn('alt="Article diagram."', rendered)

    def test_theme_image_uses_one_accessible_label(self):
        value = article(
            image="assets/blog/article.svg",
            image_dark="assets/blog/article-dark.svg",
            image_alt="Article diagram.",
        )
        rendered_article = render_blog.render_article(value)
        rendered_index = render_blog.render_index([value])

        for rendered in (rendered_article, rendered_index):
            self.assertIn('class="blog-theme-image has-dark', rendered)
            self.assertIn('aria-label="Article diagram."', rendered)
            self.assertIn("article.svg", rendered)
            self.assertIn("article-dark.svg", rendered)
            self.assertNotIn('alt="Article diagram."', rendered)

    def test_html_and_markdown_indexes_share_intro(self):
        articles = [article()]
        rendered_html = render_blog.render_index(articles)
        rendered_markdown = render_blog.render_index_markdown(articles)

        self.assertIn(render_blog.BLOG_INDEX_INTRO, rendered_html)
        self.assertIn(render_blog.BLOG_INDEX_INTRO, rendered_markdown)

    def test_wide_page_head_marks_listing_shell(self):
        page = render_blog.sitelib.page_head(
            "Blog - Ze", "Description", "../", wide=True
        )
        render_blog.sitelib.page_foot("../")

        self.assertIn('<main id="top" class="site-main-wide"', page)


class ArticleHeadingTest(unittest.TestCase):
    def test_fenced_heading_is_code_not_toc_entry(self):
        rendered = render_blog.render_article(
            article(
                body="""## Real section

```text
## literal protocol marker
```

## Second section

Body.

## Third section

Body.
"""
            )
        )

        self.assertIn('href="#real-section"', rendered)
        self.assertNotIn('href="#literal-protocol-marker"', rendered)
        self.assertIn("## literal protocol marker", rendered)
        self.assertNotIn('&lt;h2 id="literal-protocol-marker"', rendered)

    def test_explicit_section_reveal_survives_markdown_rendering(self):
        rendered = render_blog.render_article(
            article(
                body="""## Real section

<p class="blog-section-reveal">Dedicated section context.</p>

The ordinary first paragraph stays ordinary.

## Second section

Body.

## Third section

Body.
"""
            )
        )

        self.assertEqual(rendered.count('class="blog-section-reveal"'), 1)
        self.assertIn(
            '<p class="blog-section-reveal">Dedicated section context.</p>',
            rendered,
        )
        self.assertIn("<p>The ordinary first paragraph stays ordinary.</p>", rendered)


if __name__ == "__main__":
    unittest.main()
