package tui

import (
	"fmt"
	"strings"
)

func parseFrontmatter(text string) (title string, tags []string, content string) {
	if !strings.HasPrefix(text, "---\n") {
		return "", nil, text
	}
	rest := text[4:]
	end := strings.Index(rest, "\n---")
	if end == -1 {
		return "", nil, text
	}
	fm := rest[:end]
	content = strings.TrimPrefix(rest[end+4:], "\n")
	content = strings.TrimPrefix(content, "\n")
	title, tags = parseFMFields(fm)
	return title, tags, content
}

func parseFMFields(fm string) (title string, tags []string) {
	for _, line := range strings.Split(fm, "\n") {
		if strings.HasPrefix(line, "title:") {
			title = strings.TrimSpace(strings.TrimPrefix(line, "title:"))
		} else if strings.HasPrefix(line, "tags:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "tags:"))
			for _, p := range strings.Split(val, ",") {
				t := strings.TrimSpace(p)
				if t != "" {
					tags = append(tags, t)
				}
			}
		}
	}
	return title, tags
}

func writeFrontmatter(title string, tags []string, content string) string {
	return fmt.Sprintf("---\ntitle: %s\ntags: %s\n---\n\n%s", title, strings.Join(tags, ", "), content)
}
