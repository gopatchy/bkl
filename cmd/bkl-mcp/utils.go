package main

import (
	"io/fs"
	"os"
	"testing/fstest"
)

func createTestFS(fileSystem map[string]string) (fs.FS, error) {
	fsys := fstest.MapFS{}
	for filename, content := range fileSystem {
		fsys[filename] = &fstest.MapFile{
			Data: []byte(content),
		}
	}

	return fsys, nil
}

func getFileSystem(fileSystem map[string]string) (fs.FS, error) {
	if fileSystem != nil {
		return createTestFS(fileSystem)
	}
	return os.DirFS("/"), nil
}
