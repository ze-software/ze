// Design: website/style-guide/index.html -- card colors identify subjects across page families.
package site

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// TestCardCategoriesFollowTheirSubjects renders authored cards twice in opposite
// orders, so an insertion or sort cannot silently change a subject's color.
func TestCardCategoriesFollowTheirSubjects(t *testing.T) {
	source := blogPostsFixture(t, map[string]string{
		"routing.md": "---\ntitle: Routing article\nauthor: Writer\ncategory: routing\n---\nBody.",
		"operate.md": "---\ntitle: Operator article\nauthor: Writer\ncategory: operate\n---\nBody.",
		"mixed.md":   "---\ntitle: Mixed article\nauthor: Writer\n---\nBody.",
	})
	articles, err := loadBlogArticles(source)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		page := blogIndexBody(articles)
		for slug, category := range map[string]string{"routing": "routing", "operate": "operate", "mixed": "meta"} {
			assertCardCategory(t, page, slug+"/", category)
		}
		if got := strings.Count(page, `<article class="card card-post blog-card`); got != len(articles) {
			t.Errorf("the index renders %d cards for %d articles", got, len(articles))
		}
		slices.Reverse(articles)
	}
	cards := []audienceCard{
		{Title: "Operate", Body: "First", Category: categoryOperate, Link: &audienceLink{Href: "operate/", Label: "Open"}},
		{Title: "Routing", Body: "Second", Category: categoryRouting, Link: &audienceLink{Href: "routing/", Label: "Open"}},
		{Title: "Mixed", Body: "Third", Link: &audienceLink{Href: "mixed/", Label: "Open"}},
	}
	for range 2 {
		page, err := audienceCards(cards)
		if err != nil {
			t.Fatal(err)
		}
		for slug, category := range map[string]string{"routing": "routing", "operate": "operate", "mixed": "meta"} {
			assertCardCategory(t, page, slug+"/", category)
		}
		slices.Reverse(cards)
	}
}

// assertCardCategory checks the card that owns a link, rather than a class
// anywhere on the page or at a fixed position in a grid.
func assertCardCategory(t *testing.T, page, href, category string) {
	t.Helper()
	pattern := regexp.MustCompile(`(?s)<article class="([^"]+)"[^>]*>(.*?)</article>`)
	for _, card := range pattern.FindAllStringSubmatch(page, -1) {
		if !strings.Contains(card[2], `href="`+href+`"`) {
			continue
		}
		if !slices.Contains(strings.Fields(card[1]), "cat-"+category) {
			t.Errorf("card linking to %s has classes %q, want category %s", href, card[1], category)
		}
		if strings.Contains(card[1], "tone-") {
			t.Errorf("card linking to %s overrides its category with %q", href, card[1])
		}
		return
	}
	t.Errorf("no card links to %s", href)
}

// TestCardCategoryRejectsUnknownValues reaches the authored-data boundaries,
// where a typo must fail instead of becoming a class the stylesheet cannot use.
func TestCardCategoryRejectsUnknownValues(t *testing.T) {
	t.Run("audience", func(t *testing.T) {
		_, err := audienceCardHTML(&audienceCard{Title: "Unknown", Body: "Body", Category: "telepathy"})
		if err == nil {
			t.Fatal("the audience producer accepted an unknown category")
		}
		if !strings.Contains(err.Error(), "telepathy") {
			t.Errorf("the refusal must name the invalid category: %v", err)
		}
	})
	t.Run("topic", func(t *testing.T) {
		source := t.TempDir()
		writeArtifactFile(t, source, topicsVocabularyFile, `{"tags":{"Known subject":"telepathy"}}`)
		_, err := loadTopicVocabulary(source)
		if err == nil {
			t.Fatal("the topic vocabulary accepted an unknown category")
		}
		if !strings.Contains(err.Error(), "Known subject") || !strings.Contains(err.Error(), "telepathy") {
			t.Errorf("the refusal must name the subject and invalid category: %v", err)
		}
	})
	t.Run("article", func(t *testing.T) {
		source := blogPostsFixture(t, map[string]string{"unknown.md": "---\ntitle: Unknown\nauthor: Writer\ncategory: telepathy\n---\nBody."})
		_, err := readBlogArticle(filepath.Join(source, blogSourceDirectory, "unknown.md"))
		if err == nil {
			t.Fatal("the article producer accepted an unknown category")
		}
		if !strings.Contains(err.Error(), "telepathy") {
			t.Errorf("the refusal must name the invalid category: %v", err)
		}
	})
	t.Run("news", func(t *testing.T) {
		paths := homeFixture(t)
		writeSourceData(t, paths, whatsNewDataFile, `{"title":"News","link":{"href":"project/changes/","label":"All"},"note":{"title":"Unknown","body":"Body","category":"telepathy"}}`)
		_, err := renderHome(paths)
		if err == nil {
			t.Fatal("the homepage producer accepted an unknown news category")
		}
		if !strings.Contains(err.Error(), "telepathy") {
			t.Errorf("the refusal must name the invalid category: %v", err)
		}
	})
}

// TestPluginCardCategoriesKeepTheFilterBucketsSeparate verifies known subjects
// whose navigation bucket differs from their website category.
func TestPluginCardCategoriesKeepTheFilterBucketsSeparate(t *testing.T) {
	for _, example := range []struct{ group, category, bucket string }{
		{"ike", categoryServices, bucketTunneling},
		{"mpls", categoryRouting, bucketTunneling},
		{"bgp-rpki", categoryRouting, bucketSecurity},
		{"mcp", categoryAutomate, bucketSystem},
		{"lg", categoryOperate, bucketSystem},
		{"kernel", categoryPlatform, bucketDataplane},
		{"ddos", categorySecure, bucketSecurity},
		{"flow-export", categoryObserve, bucketTelemetry},
		{"unclassified", categoryMeta, bucketSystem},
		{testHarnessGroup, categoryMeta, bucketTest},
	} {
		t.Run(example.group, func(t *testing.T) {
			groups := pluginGroups([]*pluginEntry{{
				Group: example.group, Slug: example.group,
				registryPlugin: registryPlugin{Name: example.group, Description: "A fixture plugin"},
			}})
			page := pluginGroupHTML(groups[0])
			assertCardCategory(t, page, example.group+"/", example.category)
			if !strings.Contains(page, `data-category="`+example.bucket+`"`) {
				t.Errorf("the existing %s navigation bucket must remain", example.bucket)
			}
		})
	}
}

// TestPublishedSubjectsKeepTheFeatureCatalogCategory compares actual publishing
// inputs by subject identity. Expected categories come from the feature cards,
// so this check does not maintain a second category assignment table.
func TestPublishedSubjectsKeepTheFeatureCatalogCategory(t *testing.T) {
	source := filepath.Join(repositoryRoot(t), "website")
	var features featureData
	if err := readSourceJSON(source, featuresDataFile, &features); err != nil {
		t.Fatal(err)
	}
	categories := make(map[string]string)
	for _, card := range features.Cards {
		categories[strings.ToLower(card.Title)] = card.Category
	}
	topics, err := loadTopicVocabulary(source)
	if err != nil {
		t.Fatal(err)
	}
	for topic, title := range map[string]string{
		"IPsec": "IPsec VPN", "Firewall": "Firewall", "Interfaces": "Interface Management",
		"VPP": "VPP Data Plane", "MRT": "MRT Recording", "Looking Glass": "Looking Glass", "Fleet": "Fleet Management",
	} {
		category, found := categories[strings.ToLower(title)]
		if !found {
			t.Fatalf("the canonical feature card %q is absent", title)
		}
		if topics[topic] != category {
			t.Errorf("topic %q is %q, but feature %q is %q", topic, topics[topic], title, category)
		}
	}
	var audience audienceData
	if err := readSourceJSON(source, audienceDataFile, &audience); err != nil {
		t.Fatal(err)
	}
	matched := 0
	for _, card := range append(audience.Run, audience.Who...) {
		if category, found := categories[strings.ToLower(card.Title)]; found {
			matched++
			if card.Category != category {
				t.Errorf("audience card %q is %q, but the feature is %q", card.Title, card.Category, category)
			}
		}
	}
	if matched == 0 {
		t.Fatal("no audience card exercised the shared subject contract")
	}
}

// TestWeeklyCardCategoriesFollowAllTheirTopics exercises single-subject,
// mixed and untagged weeks, including a topic beyond the visible chip budget.
func TestWeeklyCardCategoriesFollowAllTheirTopics(t *testing.T) {
	weeks := []changeWeek{
		{Slug: "2026-10-19", Topics: []changeTopic{{Category: categoryRouting}, {Category: categoryRouting}}},
		{Slug: "2026-10-12", Topics: []changeTopic{{Category: categoryRouting}, {Category: categoryOperate}}},
		{Slug: "2026-10-05"},
	}
	for range 2 {
		page := blogTeaserCards(weeks)
		for slug, category := range map[string]string{"2026-10-19": categoryRouting, "2026-10-12": categoryMeta, "2026-10-05": categoryMeta} {
			assertCardCategory(t, page, changeWeek{Slug: slug}.route(), category)
		}
		slices.Reverse(weeks)
	}
	week := changeWeek{Topics: []changeTopic{{Category: categoryRouting}, {Category: categoryRouting}, {Category: categoryRouting}, {Category: categoryRouting}, {Category: categoryOperate}}}
	if got := week.cardCategory(); got != categoryMeta {
		t.Errorf("a topic beyond the visible chips must still make the card mixed, got %q", got)
	}
}

// TestRequiredSubjectCardsRejectNeutralCategory drives the actual homepage and
// catalog producers: their seven-subject inventories must reject neutral cards
// before a homepage overview or filter can silently omit them.
func TestRequiredSubjectCardsRejectNeutralCategory(t *testing.T) {
	for _, producer := range []struct {
		name   string
		render func(Paths) ([]string, error)
	}{{"homepage", renderHome}, {"features", renderFeatures}} {
		t.Run(producer.name, func(t *testing.T) {
			paths := homeFixture(t)
			if _, err := producer.render(paths); err != nil {
				t.Fatalf("valid subject inventory failed: %v", err)
			}
			var data featureData
			if err := readSourceJSON(paths.Source, featuresDataFile, &data); err != nil {
				t.Fatal(err)
			}
			data.Cards[0].Category = categoryMeta
			content, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			writeSourceData(t, paths, featuresDataFile, string(content))
			_, err = producer.render(paths)
			if err == nil {
				t.Fatal("a neutral feature reached a seven-subject producer without refusal")
			}
			for _, named := range []string{data.Cards[0].Title, categoryMeta} {
				if !strings.Contains(err.Error(), named) {
					t.Errorf("the refusal must identify %q: %v", named, err)
				}
			}
		})
	}
	t.Run("milestones", func(t *testing.T) {
		paths := milestonesPaths(t)
		if _, err := renderMilestones(paths); err != nil {
			t.Fatalf("valid subject inventory failed: %v", err)
		}
		writeSourceData(t, paths, milestonesDataFile, `{"intro":"Intro","milestones":[{"date":"2026-10-23","title":"Neutral landmark","category":"meta","blurb":"Body"}]}`)
		_, err := renderMilestones(paths)
		if err == nil {
			t.Fatal("a neutral milestone reached a seven-subject filter without refusal")
		}
		for _, named := range []string{"Neutral landmark", categoryMeta} {
			if !strings.Contains(err.Error(), named) {
				t.Errorf("the refusal must identify %q: %v", named, err)
			}
		}
	})
}
