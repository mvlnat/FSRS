package migrations

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
)

type Script struct {
	Name string
	SQL  string
}

//go:embed *.sql
var files embed.FS

func OrderedScripts() ([]Script, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	scripts := make([]Script, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		content, err := files.ReadFile(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read embedded migration %s: %w", entry.Name(), err)
		}

		scripts = append(scripts, Script{
			Name: entry.Name(),
			SQL:  string(content),
		})
	}

	return scripts, nil
}
