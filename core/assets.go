package core

import (
	"errors"
	"io/fs"
	"path"
	"strings"
	"testing/fstest"

	bashassets "github.com/nexssp/lang/languages/bash/assets"
	golangassets "github.com/nexssp/lang/languages/golang/assets"
	javascriptassets "github.com/nexssp/lang/languages/javascript/assets"
	powershellassets "github.com/nexssp/lang/languages/powershell/assets"
	pythonassets "github.com/nexssp/lang/languages/python/assets"
	rustassets "github.com/nexssp/lang/languages/rust/assets"
	shellassets "github.com/nexssp/lang/languages/shell/assets"
	typescriptassets "github.com/nexssp/lang/languages/typescript/assets"
	wslassets "github.com/nexssp/lang/languages/wsl/assets"
)

// Each language package owns its profile and template files. The runtime
// combines them into the long-standing virtual runners/ layout for callers.
var runnerAssetSources = []fs.FS{
	pythonassets.Files,
	golangassets.Files,
	javascriptassets.Files,
	rustassets.Files,
	typescriptassets.Files,
	shellassets.Files,
	bashassets.Files,
	wslassets.Files,
	powershellassets.Files,
}

var runnersFS = buildRunnerFiles()

func buildRunnerFiles() fstest.MapFS {
	files := fstest.MapFS{}
	copyFile := func(target string, data []byte) {
		if _, exists := files[target]; exists {
			panic("core: duplicate embedded runner asset " + target)
		}
		files[target] = &fstest.MapFile{Data: data}
	}

	for _, sourceFS := range runnerAssetSources {
		if err := fs.WalkDir(sourceFS, "profiles", func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			data, err := fs.ReadFile(sourceFS, name)
			if err != nil {
				return err
			}
			copyFile(path.Join("runners", path.Base(name)), data)
			return nil
		}); err != nil {
			panic("core: map embedded runner profiles: " + err.Error())
		}

		if err := fs.WalkDir(sourceFS, "templates", func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			data, err := fs.ReadFile(sourceFS, name)
			if err != nil {
				return err
			}
			relative := strings.TrimPrefix(name, "templates/")
			relative = strings.TrimSuffix(relative, ".tmpl")
			copyFile(path.Join("runners", "templates", relative), data)
			return nil
		}); err != nil && !errors.Is(err, fs.ErrNotExist) {
			panic("core: map embedded runner templates: " + err.Error())
		}
	}
	return files
}
