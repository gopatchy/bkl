package bkl

import (
	_ "embed"
	"fmt"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

//go:embed tests.toml
var testsData []byte

//go:embed docs/index.yaml
var sectionsData []byte

//go:embed docs/k8s.yaml
var k8sData []byte

type TestCase struct {
	Description string            `toml:"description"`
	Eval        []string          `toml:"eval"`
	Format      string            `toml:"format"`
	Expected    string            `toml:"expected,omitempty"`
	Files       map[string]string `toml:"files"`
	Errors      []string          `toml:"errors,omitempty"`
	RootPath    string            `toml:"rootPath,omitempty"`
	Env         map[string]string `toml:"env,omitempty"`
	Diff        bool              `toml:"diff,omitempty"`
	Intersect   bool              `toml:"intersect,omitempty"`
	Required    bool              `toml:"required,omitempty"`
	Benchmark   bool              `toml:"benchmark,omitempty"`
}

type DocSection struct {
	ID     string    `yaml:"id"`
	Title  string    `yaml:"title"`
	Items  []DocItem `yaml:"items"`
	Source string    `yaml:"-"`
}

type DocItem struct {
	Content    string         `yaml:"content,omitempty"`
	Example    *DocExample    `yaml:"example,omitempty"`
	Code       *DocLayer      `yaml:"code,omitempty"`
	SideBySide *DocSideBySide `yaml:"side_by_side,omitempty"`
}

type DocSideBySide struct {
	Left  DocLayer `yaml:"left"`
	Right DocLayer `yaml:"right"`
}

type DocExample struct {
	Operation string     `yaml:"operation,omitempty"`
	Layers    []DocLayer `yaml:"layers,omitempty"`
	Result    DocLayer   `yaml:"result,omitempty"`
}

type DocLayer struct {
	Label      string   `yaml:"label,omitempty"`
	Code       string   `yaml:"code"`
	Highlights []string `yaml:"highlights,omitempty"`
	Languages  [][]any  `yaml:"languages,omitempty"`
	Expandable bool     `yaml:"expandable,omitempty"`
	Collapsed  bool     `yaml:"collapsed,omitempty"`
}

func GetTests() (map[string]*TestCase, error) {
	var tests map[string]*TestCase
	if err := toml.Unmarshal(testsData, &tests); err != nil {
		return nil, err
	}
	return tests, nil
}

func GetDocSections() ([]DocSection, error) {
	var allSections []DocSection

	files := map[string][]byte{
		"index": sectionsData,
		"k8s":   k8sData,
	}

	for name, data := range files {
		var sections []DocSection
		if err := yaml.Unmarshal(data, &sections); err != nil {
			return nil, fmt.Errorf("failed to unmarshal %s.yaml: %w", name, err)
		}
		for i := range sections {
			sections[i].Source = name
		}
		allSections = append(allSections, sections...)
	}

	return allSections, nil
}
