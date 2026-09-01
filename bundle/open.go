package bundle

import (
	"archive/zip"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/GoCraft-MC/gocraft-abi/command"
)

// Open validates one archive and decodes its root plugin.toml.
func Open(bundlePath string) (Bundle, error) {
	archive, err := zip.OpenReader(bundlePath)
	if err != nil {
		return Bundle{}, fmt.Errorf("open plugin bundle %s: %w", bundlePath, err)
	}
	defer archive.Close()
	var manifestFile *zip.File
	for _, file := range archive.File {
		name := strings.ReplaceAll(file.Name, "\\", "/")
		clean := path.Clean(name)
		if path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
			return Bundle{}, fmt.Errorf("plugin bundle %s contains unsafe path %q", bundlePath, file.Name)
		}
		if clean != ManifestFileName {
			continue
		}
		if manifestFile != nil {
			return Bundle{}, fmt.Errorf("plugin bundle %s contains duplicate plugin.toml", bundlePath)
		}
		manifestFile = file
	}
	if manifestFile == nil {
		return Bundle{}, fmt.Errorf("plugin bundle %s has no root plugin.toml", bundlePath)
	}
	if manifestFile.UncompressedSize64 > maximumManifestSize {
		return Bundle{}, fmt.Errorf("plugin bundle %s: plugin.toml exceeds %d bytes", bundlePath, maximumManifestSize)
	}
	reader, err := manifestFile.Open()
	if err != nil {
		return Bundle{}, fmt.Errorf("open %s plugin.toml: %w", bundlePath, err)
	}
	manifest, decodeErr := DecodeManifest(reader)
	closeErr := reader.Close()
	if decodeErr != nil {
		return Bundle{}, fmt.Errorf("plugin bundle %s: %w", bundlePath, decodeErr)
	}
	if closeErr != nil {
		return Bundle{}, fmt.Errorf("close %s plugin.toml: %w", bundlePath, closeErr)
	}
	var commands *command.Root
	if manifest.CommandTree != "" {
		encoded, err := readBundleEntry(archive.File, manifest.CommandTree, maximumCommandTreeSize)
		if err != nil {
			return Bundle{}, fmt.Errorf("plugin bundle %s: %w", bundlePath, err)
		}
		tree, err := command.DecodeTree(encoded)
		if err != nil {
			return Bundle{}, fmt.Errorf("plugin bundle %s command tree: %w", bundlePath, err)
		}
		commands = &tree
	}
	absolutePath, err := filepath.Abs(bundlePath)
	if err != nil {
		return Bundle{}, fmt.Errorf("resolve plugin bundle %s: %w", bundlePath, err)
	}
	return Bundle{Path: absolutePath, Manifest: manifest, Commands: commands}, nil
}
