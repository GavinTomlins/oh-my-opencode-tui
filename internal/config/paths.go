package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Paths struct {
	Home             string
	Root             string
	ActiveConfig     string
	Profiles         string
	Providers        string
	UIState          string
	BackupDir        string
	DefaultSchemaURL string
}

func DiscoverPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve home directory: %w", err)
	}
	root := filepath.Join(home, ".config", "opencode")
	return Paths{
		Home:             home,
		Root:             root,
		ActiveConfig:     filepath.Join(root, "oh-my-opencode.json"),
		Profiles:         filepath.Join(root, "config", "oh-my-opencode-profiles.json"),
		Providers:        filepath.Join(root, "opencode.json"),
		UIState:          filepath.Join(root, "config", "oh-my-opencode-tui.json"),
		BackupDir:        filepath.Join(root, "config", "backups"),
		DefaultSchemaURL: "https://raw.githubusercontent.com/code-yeongyu/oh-my-opencode/master/assets/oh-my-opencode.schema.json",
	}, nil
}
