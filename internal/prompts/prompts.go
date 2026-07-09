package prompts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
)

type Prompt struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

var placeholderRe = regexp.MustCompile(`\{\{([^}]+)\}\}`)

func (p Prompt) Placeholders() []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range placeholderRe.FindAllStringSubmatch(p.Text, -1) {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

func (p Prompt) Fill(values map[string]string) string {
	return placeholderRe.ReplaceAllStringFunc(p.Text, func(match string) string {
		name := match[2 : len(match)-2]
		if v, ok := values[name]; ok {
			return v
		}
		return match
	})
}

func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "midnight-director", "prompts.json")
}

func Load(path string) ([]Prompt, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ps []Prompt
	return ps, json.Unmarshal(data, &ps)
}
