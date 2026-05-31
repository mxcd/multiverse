package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// BindingFile is the per-directory binding mapping a working directory subtree to
// the brains it reads from and writes to. Walked up from the cwd like .git.
const BindingFile = ".multi.yaml"

// Binding is the on-disk shape of a .multi.yaml.
type Binding struct {
	Sources []string `yaml:"sources"`
	Targets []string `yaml:"targets,omitempty"`
}

// FindBinding walks up from the cwd for the nearest binding file, returning its
// path and contents, or ("", nil, nil) when none exists.
func FindBinding() (string, *Binding, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", nil, err
	}
	for {
		p := filepath.Join(dir, BindingFile)
		if isFile(p) {
			bnd, err := readBinding(p)
			if err != nil {
				return "", nil, err
			}
			return p, bnd, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil, nil
		}
		dir = parent
	}
}

// ReadBindingAt reads the binding in a specific directory, or nil if absent.
func ReadBindingAt(dir string) (*Binding, error) {
	p := filepath.Join(dir, BindingFile)
	if !isFile(p) {
		return nil, nil
	}
	return readBinding(p)
}

func readBinding(p string) (*Binding, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var bnd Binding
	if err := yaml.Unmarshal(data, &bnd); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	return &bnd, nil
}

// WriteBinding writes a binding into dir, returning the file path.
func WriteBinding(dir string, bnd Binding) (string, error) {
	p := filepath.Join(dir, BindingFile)
	data, err := yaml.Marshal(bnd)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return "", err
	}
	return p, nil
}

func isFile(p string) bool { info, err := os.Stat(p); return err == nil && !info.IsDir() }
