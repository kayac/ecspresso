package ecspresso

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// DocsOption defines CLI options for the docs subcommand.
type DocsOption struct {
	Article string `help:"article name to display" default:"readme" enum:"readme,v1-v2,v2-v3"`
	List    bool   `help:"list available articles" default:"false"`
	Index   bool   `help:"show table of contents" default:"false"`
	Search  string `help:"search keyword in documents" default:""`
	JSON    bool   `help:"output in JSON format" default:"false" name:"json"`
}

type docsSection struct {
	Level   int    `json:"level"`
	Title   string `json:"title"`
	Content string `json:"content,omitempty"`
	Line    int    `json:"line"`
}

type docsOutput struct {
	Article  string        `json:"article"`
	Mode     string        `json:"mode"`
	Query    string        `json:"query,omitempty"`
	Sections []docsSection `json:"sections"`
}

type docsArticle struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

var articles = []docsArticle{
	{Name: "readme", Description: "ecspresso README"},
	{Name: "v1-v2", Description: "Migration guide from v1 to v2"},
	{Name: "v2-v3", Description: "Migration guide from v2 to v3"},
}

func dispatchDocs(ctx context.Context, opt *DocsOption) error {
	return Docs(ctx, *opt)
}

// Docs shows embedded documentation.
func Docs(_ context.Context, opt DocsOption) error {
	jsonOutput := opt.JSON || logFormat == logFormatJSON

	if opt.List {
		return docsList(jsonOutput)
	}

	content, err := getArticle(opt.Article)
	if err != nil {
		return err
	}

	switch {
	case opt.Index:
		return docsIndex(opt.Article, content, jsonOutput)
	case opt.Search != "":
		return docsSearch(opt.Article, content, opt.Search, jsonOutput)
	default:
		return docsFull(opt.Article, content, jsonOutput)
	}
}

func getArticle(name string) (string, error) {
	switch name {
	case "readme":
		return readmeContent, nil
	case "v1-v2":
		return docsV1V2Content, nil
	case "v2-v3":
		return docsV2V3Content, nil
	default:
		return "", fmt.Errorf("unknown article: %s", name)
	}
}

func docsList(jsonOutput bool) error {
	if !jsonOutput {
		var buf strings.Builder
		for _, a := range articles {
			fmt.Fprintf(&buf, "%s\t%s\n", a.Name, a.Description)
		}
		_, err := io.WriteString(os.Stdout, buf.String())
		return err
	}
	return writeJSON(articles)
}

func docsFull(article, content string, jsonOutput bool) error {
	if !jsonOutput {
		_, err := io.WriteString(os.Stdout, content)
		return err
	}
	sections := parseSections(content)
	out := docsOutput{
		Article:  article,
		Mode:     "full",
		Sections: sections,
	}
	return writeJSON(out)
}

func docsIndex(article, content string, jsonOutput bool) error {
	sections := parseSections(content)

	if !jsonOutput {
		var buf strings.Builder
		for _, s := range sections {
			indent := strings.Repeat("  ", s.Level-1)
			prefix := strings.Repeat("#", s.Level)
			fmt.Fprintf(&buf, "%s%s %s (L%d)\n", indent, prefix, s.Title, s.Line)
		}
		_, err := io.WriteString(os.Stdout, buf.String())
		return err
	}

	indexSections := make([]docsSection, len(sections))
	for i, s := range sections {
		indexSections[i] = docsSection{
			Level: s.Level,
			Title: s.Title,
			Line:  s.Line,
		}
	}
	out := docsOutput{
		Article:  article,
		Mode:     "index",
		Sections: indexSections,
	}
	return writeJSON(out)
}

func docsSearch(article, content, query string, jsonOutput bool) error {
	sections := parseSections(content)
	lowerQuery := strings.ToLower(query)

	var matched []docsSection
	for _, s := range sections {
		if strings.Contains(strings.ToLower(s.Content), lowerQuery) {
			matched = append(matched, s)
		}
	}

	if len(matched) == 0 {
		return fmt.Errorf("no sections found matching %q in article %q", query, article)
	}

	if !jsonOutput {
		var buf strings.Builder
		for i, s := range matched {
			if i > 0 {
				buf.WriteString("\n---\n\n")
			}
			buf.WriteString(strings.TrimRight(s.Content, "\n"))
			buf.WriteString("\n")
		}
		_, err := io.WriteString(os.Stdout, buf.String())
		return err
	}

	out := docsOutput{
		Article:  article,
		Mode:     "search",
		Query:    query,
		Sections: matched,
	}
	return writeJSON(out)
}

func writeJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal json: %w", err)
	}
	b = append(b, '\n')
	_, err = os.Stdout.Write(b)
	return err
}

// parseSections parses markdown content into leaf sections.
// Each section starts at a heading (line beginning with #) and extends
// until the next heading of any level. Lines inside fenced code blocks
// are not treated as headings.
func parseSections(content string) []docsSection {
	lines := strings.Split(content, "\n")
	var sections []docsSection
	var current *docsSection
	var body strings.Builder
	inCodeBlock := false

	for i, line := range lines {
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
		}

		if !inCodeBlock && strings.HasPrefix(line, "#") {
			// flush previous section
			if current != nil {
				current.Content = body.String()
				sections = append(sections, *current)
			}

			level := 0
			for _, c := range line {
				if c == '#' {
					level++
				} else {
					break
				}
			}
			title := strings.TrimSpace(line[level:])

			current = &docsSection{
				Level: level,
				Title: title,
				Line:  i + 1, // 1-based
			}
			body.Reset()
			body.WriteString(line)
			body.WriteString("\n")
		} else if current != nil {
			body.WriteString(line)
			body.WriteString("\n")
		}
	}
	// flush last section
	if current != nil {
		current.Content = body.String()
		sections = append(sections, *current)
	}

	return sections
}
