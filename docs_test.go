package ecspresso_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kayac/ecspresso/v2"
)

func TestParseSections(t *testing.T) {
	content := "# Title\n\nIntro text.\n\n## Section A\n\nBody A.\n\n### Subsection A1\n\nBody A1.\n\n## Section B\n\nBody B.\n"
	sections := ecspresso.ParseSections(content)

	if len(sections) != 4 {
		t.Fatalf("expected 4 sections, got %d", len(sections))
	}

	want := []struct {
		level int
		title string
		line  int
	}{
		{1, "Title", 1},
		{2, "Section A", 5},
		{3, "Subsection A1", 9},
		{2, "Section B", 13},
	}

	for i, w := range want {
		s := sections[i]
		if s.Level != w.level {
			t.Errorf("section[%d] level: want %d, got %d", i, w.level, s.Level)
		}
		if s.Title != w.title {
			t.Errorf("section[%d] title: want %q, got %q", i, w.title, s.Title)
		}
		if s.Line != w.line {
			t.Errorf("section[%d] line: want %d, got %d", i, w.line, s.Line)
		}
	}

	// verify content of first section includes intro text
	if !strings.Contains(sections[0].Content, "Intro text.") {
		t.Errorf("section[0] content should contain 'Intro text.', got %q", sections[0].Content)
	}

	// verify section A does not contain subsection A1 content
	if strings.Contains(sections[1].Content, "Body A1.") {
		t.Errorf("section[1] should not contain 'Body A1.' (leaf section)")
	}
}

func TestParseSectionsCodeBlock(t *testing.T) {
	content := "## Real Section\n\n```yaml\n# this is a comment not a heading\nfoo: bar\n```\n\n## Next Section\n\nBody.\n"
	sections := ecspresso.ParseSections(content)

	if len(sections) != 2 {
		t.Fatalf("expected 2 sections (code block # should be ignored), got %d", len(sections))
	}

	if sections[0].Title != "Real Section" {
		t.Errorf("section[0] title: want %q, got %q", "Real Section", sections[0].Title)
	}
	if sections[1].Title != "Next Section" {
		t.Errorf("section[1] title: want %q, got %q", "Next Section", sections[1].Title)
	}

	// verify the code block content is in the first section
	if !strings.Contains(sections[0].Content, "# this is a comment not a heading") {
		t.Errorf("section[0] should contain code block content")
	}
}

func TestParseSectionsEmpty(t *testing.T) {
	sections := ecspresso.ParseSections("")
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for empty content, got %d", len(sections))
	}
}

func TestParseSectionsReadme(t *testing.T) {
	content := ecspresso.ReadmeContent
	sections := ecspresso.ParseSections(content)

	if len(sections) == 0 {
		t.Fatal("expected non-zero sections from README.md")
	}

	// verify well-known headings exist
	titles := make(map[string]bool)
	for _, s := range sections {
		titles[s.Title] = true
	}

	for _, expected := range []string{"ecspresso", "Install", "Usage", "Plugins"} {
		if !titles[expected] {
			t.Errorf("expected heading %q not found in README.md sections", expected)
		}
	}

	// verify first section is the top-level heading
	if sections[0].Level != 1 || sections[0].Title != "ecspresso" {
		t.Errorf("first section should be '# ecspresso', got level=%d title=%q", sections[0].Level, sections[0].Title)
	}
}

func TestDocsSearch(t *testing.T) {
	content := ecspresso.ReadmeContent
	sections := ecspresso.ParseSections(content)

	query := "fargate"
	lowerQuery := strings.ToLower(query)
	var matched []string
	for _, s := range sections {
		if strings.Contains(strings.ToLower(s.Content), lowerQuery) {
			matched = append(matched, s.Title)
		}
	}

	if len(matched) == 0 {
		t.Fatal("expected at least one section matching 'fargate'")
	}

	// verify well-known fargate sections
	found := false
	for _, title := range matched {
		if strings.Contains(strings.ToLower(title), "fargate") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a section with 'fargate' in the title among matched sections: %v", matched)
	}
}

func TestDocsSearchNoMatch(t *testing.T) {
	content := "# Title\n\nSome content.\n"
	sections := ecspresso.ParseSections(content)

	query := "zzz_nonexistent_keyword_zzz"
	lowerQuery := strings.ToLower(query)
	var matched int
	for _, s := range sections {
		if strings.Contains(strings.ToLower(s.Content), lowerQuery) {
			matched++
		}
	}

	if matched != 0 {
		t.Errorf("expected 0 matches for non-existent keyword, got %d", matched)
	}
}

func TestDocsOptionDefault(t *testing.T) {
	want := &ecspresso.DocsOption{
		Article: "readme",
		List:    false,
		Index:   false,
		Search:  "",
		JSON:    false,
	}
	args := []string{"docs"}
	_, opts, _, err := ecspresso.ParseCLI(args)
	if err != nil {
		t.Fatal(err)
	}
	got := opts.ForSubCommand("docs")
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("DocsOption mismatch (-want +got):\n%s", diff)
	}
}

func TestDocsListText(t *testing.T) {
	err := ecspresso.Docs(t.Context(), ecspresso.DocsOption{List: true})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDocsListJSON(t *testing.T) {
	err := ecspresso.Docs(t.Context(), ecspresso.DocsOption{List: true, JSON: true})
	if err != nil {
		t.Fatal(err)
	}
}
