package tui

import (
	"fmt"
	"strings"
	"time"
)

func parseFrontmatter(text string) (title string, tags []string, createdAt time.Time, content string) {
	if !strings.HasPrefix(text, "---\n") {
		return "", nil, time.Time{}, text
	}
	rest := text[4:]
	end := strings.Index(rest, "\n---")
	if end == -1 {
		return "", nil, time.Time{}, text
	}
	fm := rest[:end]
	content = strings.TrimPrefix(rest[end+4:], "\n")
	content = strings.TrimPrefix(content, "\n")
	title, tags, createdAt = parseFMFields(fm)
	return title, tags, createdAt, content
}

func parseFMFields(fm string) (title string, tags []string, createdAt time.Time) {
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
		} else if strings.HasPrefix(line, "created:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "created:"))
			if t, err := time.Parse("2006-01-02", val); err == nil {
				createdAt = t.UTC()
			}
		}
	}
	return title, tags, createdAt
}

func writeFrontmatter(title string, tags []string, createdAt time.Time, content string) string {
	created := ""
	if !createdAt.IsZero() {
		created = fmt.Sprintf("\ncreated: %s", createdAt.Format("2006-01-02"))
	}
	return fmt.Sprintf("---\ntitle: %s\ntags: %s%s\n---\n\n%s", title, strings.Join(tags, ", "), created, content)
}
