package ai

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const SummarizeTimeout = 30 * time.Second
const maxPaneLines = 80
const minPaneLines = 3

var candidates = []string{"claude", "gemini", "codex"}

func Detect() string {
	for _, name := range candidates {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

func Summarize(bin, paneText string) (string, error) {
	lines := strings.Split(strings.TrimSpace(paneText), "\n")

	var nonEmpty []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			nonEmpty = append(nonEmpty, l)
		}
	}
	if len(nonEmpty) < minPaneLines {
		return "", fmt.Errorf("not enough output to summarize")
	}
	if len(nonEmpty) > maxPaneLines {
		nonEmpty = nonEmpty[len(nonEmpty)-maxPaneLines:]
	}

	prompt := "Summarize what happened in this terminal output in ONE sentence, " +
		"max 15 words, focus on the result. Reply with just the sentence, no punctuation at the end:\n\n" +
		strings.Join(nonEmpty, "\n")

	ctx, cancel := context.WithTimeout(context.Background(), SummarizeTimeout)
	defer cancel()

	name := filepath.Base(bin)
	var cmd *exec.Cmd
	switch name {
	case "claude":
		cmd = exec.CommandContext(ctx, bin, "-p", prompt)
	case "gemini":
		cmd = exec.CommandContext(ctx, bin, prompt)
	default:
		cmd = exec.CommandContext(ctx, bin, "-p", prompt)
	}

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ai error: %w", err)
	}

	result := strings.TrimSpace(string(out))
	if idx := strings.IndexByte(result, '\n'); idx > 0 {
		result = result[:idx]
	}
	result = strings.TrimRight(result, ".!?,")
	return result, nil
}
