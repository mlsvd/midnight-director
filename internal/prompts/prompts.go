package prompts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
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

// DefaultFilesDir returns the directory scanned for standalone .txt/.md
// prompt files, alongside the prompts.json from DefaultPath.
func DefaultFilesDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "midnight-director", "prompts")
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

var promptFileExts = map[string]bool{".txt": true, ".md": true}

// LoadFiles loads one prompt per .txt/.md file in dir. The file name without
// its extension becomes the prompt's name; the file's contents become its
// text. A missing directory yields no prompts rather than an error.
func LoadFiles(dir string) ([]Prompt, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() || !promptFileExts[filepath.Ext(e.Name())] {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	var ps []Prompt
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		ps = append(ps, Prompt{
			Name: strings.TrimSuffix(name, filepath.Ext(name)),
			Text: strings.TrimRight(string(data), "\n"),
		})
	}
	return ps, nil
}

// LoadAll loads prompts.json from jsonPath, then appends any .txt prompt
// files found in filesDir.
func LoadAll(jsonPath, filesDir string) ([]Prompt, error) {
	ps, err := Load(jsonPath)
	if err != nil {
		return nil, err
	}
	fps, err := LoadFiles(filesDir)
	if err != nil {
		return nil, err
	}
	return append(ps, fps...), nil
}
