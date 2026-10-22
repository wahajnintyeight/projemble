package agent

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

//go:embed skilldocs
var skillDocs embed.FS

const skillDocsRoot = "skilldocs"

type skillSummary struct {
	Name, Description string
}

func packagedSkills() []skillSummary {
	entries, err := fs.ReadDir(skillDocs, skillDocsRoot)
	if err != nil {
		return nil
	}
	var skills []skillSummary
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := fs.ReadFile(skillDocs, path.Join(skillDocsRoot, entry.Name(), "SKILL.md"))
		if err != nil {
			continue
		}
		skills = append(skills, skillSummary{Name: entry.Name(), Description: skillFrontMatter(string(data), "description")})
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills
}

func skillFrontMatter(document, key string) string {
	if !strings.HasPrefix(document, "---") {
		return ""
	}
	header, _, found := strings.Cut(strings.TrimPrefix(document, "---"), "\n---")
	if !found {
		return ""
	}
	for _, line := range strings.Split(header, "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), key+":"); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func readSkill(name string) (string, error) {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, "..") {
		return "", errors.New("skill name must be a packaged skill or a skill-relative .md path")
	}
	file := path.Join(skillDocsRoot, name)
	if !strings.Contains(name, "/") {
		file = path.Join(file, "SKILL.md")
	} else if path.Ext(name) != ".md" {
		return "", errors.New("skill references must be .md files")
	}
	data, err := fs.ReadFile(skillDocs, file)
	if err != nil {
		var names []string
		for _, skill := range packagedSkills() {
			names = append(names, skill.Name)
		}
		return "", fmt.Errorf("unknown skill document %q; packaged skills: %s", name, strings.Join(names, ", "))
	}
	return string(data), nil
}

func packagedSkillIndex() string {
	var index strings.Builder
	for _, skill := range packagedSkills() {
		fmt.Fprintf(&index, "\n- %s: %s", skill.Name, truncateUTF8(skill.Description, 400))
	}
	if index.Len() == 0 {
		return ""
	}
	return "Packaged skills (load with read_skill before work they cover; references load with read_skill \"<skill>/references/<file>.md\"):" + index.String()
}
