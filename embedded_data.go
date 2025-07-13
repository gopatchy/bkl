package bkl

import (
	"bytes"
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

//go:embed docs/fixit.yaml
var fixitData []byte

type TestCase struct {
	Description string            `toml:"description" json:"description"`
	Eval        []string          `toml:"eval" json:"eval"`
	Format      string            `toml:"format" json:"format"`
	Expected    string            `toml:"expected,omitempty" json:"expected,omitempty"`
	Files       map[string]string `toml:"files" json:"files"`
	Errors      []string          `toml:"errors,omitempty" json:"errors,omitempty"`
	RootPath    string            `toml:"rootPath,omitempty" json:"rootPath,omitempty"`
	Env         map[string]string `toml:"env,omitempty" json:"env,omitempty"`
	Diff        bool              `toml:"diff,omitempty" json:"diff,omitempty"`
	Intersect   bool              `toml:"intersect,omitempty" json:"intersect,omitempty"`
	Required    bool              `toml:"required,omitempty" json:"required,omitempty"`
	Compare     bool              `toml:"compare,omitempty" json:"compare,omitempty"`
	Benchmark   bool              `toml:"benchmark,omitempty" json:"benchmark,omitempty"`
	Selector    string            `toml:"selector,omitempty" json:"selector,omitempty"`
	SortPath    string            `toml:"sortPath,omitempty" json:"sortPath,omitempty"`
}

type DocSection struct {
	ID     string    `yaml:"id" json:"id"`
	Title  string    `yaml:"title" json:"title"`
	Items  []DocItem `yaml:"items" json:"items"`
	Source string    `yaml:"-" json:"-"`
}

type DocItem struct {
	Content    string         `yaml:"content,omitempty" json:"content,omitempty"`
	Example    *DocExample    `yaml:"example,omitempty" json:"example,omitempty"`
	Code       *DocLayer      `yaml:"code,omitempty" json:"code,omitempty"`
	SideBySide *DocSideBySide `yaml:"side_by_side,omitempty" json:"side_by_side,omitempty"`
}

type DocSideBySide struct {
	Left  DocLayer `yaml:"left" json:"left"`
	Right DocLayer `yaml:"right" json:"right"`
}

type DocExample struct {
	Evaluate  *DocEvaluate  `yaml:"evaluate,omitempty" json:"evaluate,omitempty"`
	Diff      *DocDiff      `yaml:"diff,omitempty" json:"diff,omitempty"`
	Intersect *DocIntersect `yaml:"intersect,omitempty" json:"intersect,omitempty"`
	Convert   *DocConvert   `yaml:"convert,omitempty" json:"convert,omitempty"`
	Fixit     *DocFixit     `yaml:"fixit,omitempty" json:"fixit,omitempty"`
	Compare   *DocCompare   `yaml:"compare,omitempty" json:"compare,omitempty"`
}

type DocEvaluate struct {
	Inputs []DocLayer `yaml:"inputs" json:"inputs"`
	Result DocLayer   `yaml:"result" json:"result"`
}

type DocDiff struct {
	Base   DocLayer `yaml:"base" json:"base"`
	Target DocLayer `yaml:"target" json:"target"`
	Result DocLayer `yaml:"result" json:"result"`
}

type DocIntersect struct {
	Inputs []DocLayer `yaml:"inputs" json:"inputs"`
	Result DocLayer   `yaml:"result" json:"result"`
}

type DocConvert struct {
	From DocLayer `yaml:"from" json:"from"`
	To   DocLayer `yaml:"to" json:"to"`
}

type DocFixit struct {
	Original DocLayer `yaml:"original,omitempty" json:"original,omitempty"`
	Bad      DocLayer `yaml:"bad" json:"bad"`
	Good     DocLayer `yaml:"good" json:"good"`
}

type DocCompare struct {
	Left   DocLayer `yaml:"left" json:"left"`
	Right  DocLayer `yaml:"right" json:"right"`
	Result DocLayer `yaml:"result" json:"result"`
}

type DocLayer struct {
	Label      string   `yaml:"label,omitempty" json:"label,omitempty"`
	Code       string   `yaml:"code" json:"code"`
	Highlights []string `yaml:"highlights,omitempty" json:"highlights,omitempty"`
	Languages  [][]any  `yaml:"languages,omitempty" json:"languages,omitempty"`
	Expandable bool     `yaml:"expandable,omitempty" json:"expandable,omitempty"`
	Collapsed  bool     `yaml:"collapsed,omitempty" json:"collapsed,omitempty"`
}

func GetTests() (map[string]*TestCase, error) {
	var tests map[string]*TestCase
	decoder := toml.NewDecoder(bytes.NewReader(testsData))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&tests); err != nil {
		return nil, err
	}
	return tests, nil
}

func GetDocSections() ([]DocSection, error) {
	var allSections []DocSection

	files := map[string][]byte{
		"index": sectionsData,
		"k8s":   k8sData,
		"fixit": fixitData,
	}

	for name, data := range files {
		var sections []DocSection
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(true)
		if err := decoder.Decode(&sections); err != nil {
			return nil, fmt.Errorf("failed to unmarshal %s.yaml: %w", name, err)
		}
		for i := range sections {
			sections[i].Source = name
		}
		allSections = append(allSections, sections...)
	}

	return allSections, nil
}
