package bkl

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"

	"github.com/gopatchy/bkl/internal/format"
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
	Description string        `toml:"description" json:"description" yaml:"description,omitempty"`
	Evaluate    *DocEvaluate  `yaml:"evaluate,omitempty" json:"evaluate,omitempty" toml:"evaluate,omitempty"`
	Diff        *DocDiff      `yaml:"diff,omitempty" json:"diff,omitempty" toml:"diff,omitempty"`
	Intersect   *DocIntersect `yaml:"intersect,omitempty" json:"intersect,omitempty" toml:"intersect,omitempty"`
	Required    *DocRequired  `yaml:"required,omitempty" json:"required,omitempty" toml:"required,omitempty"`
	Convert     *DocConvert   `yaml:"convert,omitempty" json:"convert,omitempty" toml:"convert,omitempty"`
	Fixit       *DocFixit     `yaml:"fixit,omitempty" json:"fixit,omitempty" toml:"fixit,omitempty"`
	Compare     *DocCompare   `yaml:"compare,omitempty" json:"compare,omitempty" toml:"compare,omitempty"`
	Benchmark   bool          `toml:"benchmark,omitempty" json:"benchmark,omitempty" yaml:"benchmark,omitempty"`
}

type DocEvaluate struct {
	Inputs []*DocLayer       `yaml:"inputs" json:"inputs" toml:"inputs"`
	Result DocLayer          `yaml:"result" json:"result" toml:"result"`
	Env    map[string]string `yaml:"env,omitempty" json:"env,omitempty" toml:"env,omitempty"`
	Errors []string          `yaml:"errors,omitempty" json:"errors,omitempty" toml:"errors,omitempty"`
	Root   string            `yaml:"root,omitempty" json:"root,omitempty" toml:"root,omitempty"`
	Sort   []string          `yaml:"sort,omitempty" json:"sort,omitempty" toml:"sort,omitempty"`
}

type DocDiff struct {
	Base     DocLayer `yaml:"base" json:"base" toml:"base"`
	Target   DocLayer `yaml:"target" json:"target" toml:"target"`
	Result   DocLayer `yaml:"result" json:"result" toml:"result"`
	Selector []string `yaml:"selector,omitempty" json:"selector,omitempty" toml:"selector,omitempty"`
	Errors   []string `yaml:"errors,omitempty" json:"errors,omitempty" toml:"errors,omitempty"`
}

type DocIntersect struct {
	Inputs   []*DocLayer `yaml:"inputs" json:"inputs" toml:"inputs"`
	Result   DocLayer    `yaml:"result" json:"result" toml:"result"`
	Selector []string    `yaml:"selector,omitempty" json:"selector,omitempty" toml:"selector,omitempty"`
	Errors   []string    `yaml:"errors,omitempty" json:"errors,omitempty" toml:"errors,omitempty"`
}

type DocRequired struct {
	Inputs []*DocLayer       `yaml:"inputs" json:"inputs" toml:"inputs"`
	Result DocLayer          `yaml:"result" json:"result" toml:"result"`
	Env    map[string]string `yaml:"env,omitempty" json:"env,omitempty" toml:"env,omitempty"`
	Errors []string          `yaml:"errors,omitempty" json:"errors,omitempty" toml:"errors,omitempty"`
	Root   string            `yaml:"root,omitempty" json:"root,omitempty" toml:"root,omitempty"`
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
	Left   DocLayer          `yaml:"left" json:"left" toml:"left"`
	Right  DocLayer          `yaml:"right" json:"right" toml:"right"`
	Result DocLayer          `yaml:"result" json:"result" toml:"result"`
	Env    map[string]string `yaml:"env,omitempty" json:"env,omitempty" toml:"env,omitempty"`
	Sort   []string          `yaml:"sort,omitempty" json:"sort,omitempty" toml:"sort,omitempty"`
}

type DocLayer struct {
	Label      string   `yaml:"label,omitempty" json:"label,omitempty" toml:"label,omitempty"`
	Filename   string   `yaml:"filename,omitempty" json:"filename,omitempty" toml:"filename,omitempty"`
	Code       string   `yaml:"code" json:"code" toml:"code"`
	Content    string   `yaml:"content,omitempty" json:"content,omitempty" toml:"content,omitempty"`
	Highlights []string `yaml:"highlights,omitempty" json:"highlights,omitempty" toml:"highlights,omitempty"`
	Languages  [][]any  `yaml:"languages,omitempty" json:"languages,omitempty" toml:"languages,omitempty"`
	Expandable bool     `yaml:"expandable,omitempty" json:"expandable,omitempty" toml:"expandable,omitempty"`
	Collapsed  bool     `yaml:"collapsed,omitempty" json:"collapsed,omitempty" toml:"collapsed,omitempty"`
}

func GetTests() (map[string]*DocExample, error) {
	var tests map[string]*DocExample
	decoder := toml.NewDecoder(bytes.NewReader(testsData))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&tests); err != nil {
		if derr, ok := err.(*toml.DecodeError); ok {
			row, col := derr.Position()
			return nil, fmt.Errorf("TOML decode error at row %d, column %d: %w", row, col, err)
		}
		if smerr, ok := err.(*toml.StrictMissingError); ok {
			return nil, fmt.Errorf("TOML strict mode error: %s", smerr.String())
		}
		return nil, fmt.Errorf("TOML decode error (type %T): %w", err, err)
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

func GetAllTests() (map[string]*DocExample, error) {
	tests, err := GetTests()
	if err != nil {
		return nil, fmt.Errorf("failed to get tests: %w", err)
	}

	sections, err := GetDocSections()
	if err != nil {
		return nil, fmt.Errorf("failed to get doc sections: %w", err)
	}

	for _, section := range sections {
		i := 0

		for _, item := range section.Items {
			if item.Example == nil {
				continue
			}

			testName := fmt.Sprintf("%s_example%d", section.ID, i)
			tests[testName] = item.Example
			i++
		}
	}

	return tests, nil
}

func (dl *DocLayer) ConvertCodeBlocks(targetFormat string) bool {
	if dl == nil || dl.Code == "" {
		return false
	}

	var sourceFormat string
	if dl.Filename != "" {
		if idx := strings.LastIndex(dl.Filename, "."); idx != -1 {
			sourceFormat = dl.Filename[idx+1:]
		}
	} else if len(dl.Languages) > 0 {
		if len(dl.Languages[0]) > 0 {
			if lang, ok := dl.Languages[0][0].(string); ok {
				sourceFormat = lang
			}
		}
		if sourceFormat == "" && len(dl.Languages[0]) > 1 {
			if lang, ok := dl.Languages[0][1].(string); ok {
				sourceFormat = lang
			}
		}
	}

	if sourceFormat == "" || sourceFormat == targetFormat {
		return false
	}

	sourceHandler, err := format.Get(sourceFormat)
	if err != nil {
		return false
	}

	docs, err := sourceHandler.UnmarshalStream([]byte(dl.Code))
	if err != nil {
		return false
	}

	targetHandler, err := format.Get(targetFormat)
	if err != nil {
		return false
	}

	targetBytes, err := targetHandler.MarshalStream(docs)
	if err != nil {
		return false
	}

	dl.Code = string(targetBytes)
	dl.Content = string(targetBytes)

	if len(dl.Languages) > 0 {
		if len(dl.Languages[0]) > 0 {
			dl.Languages[0][0] = targetFormat
		}
		if len(dl.Languages[0]) > 1 {
			dl.Languages[0][1] = targetFormat
		}
	}

	return true
}

func (de *DocEvaluate) ConvertCodeBlocks(targetFormat string) bool {
	if de == nil {
		return false
	}

	converted := false
	for _, input := range de.Inputs {
		if input.ConvertCodeBlocks(targetFormat) {
			converted = true
		}
	}
	if de.Result.ConvertCodeBlocks(targetFormat) {
		converted = true
	}
	return converted
}

func (dd *DocDiff) ConvertCodeBlocks(targetFormat string) bool {
	if dd == nil {
		return false
	}

	converted := false
	if dd.Base.ConvertCodeBlocks(targetFormat) {
		converted = true
	}
	if dd.Target.ConvertCodeBlocks(targetFormat) {
		converted = true
	}
	if dd.Result.ConvertCodeBlocks(targetFormat) {
		converted = true
	}
	return converted
}

func (di *DocIntersect) ConvertCodeBlocks(targetFormat string) bool {
	if di == nil {
		return false
	}

	converted := false
	for _, input := range di.Inputs {
		if input.ConvertCodeBlocks(targetFormat) {
			converted = true
		}
	}
	if di.Result.ConvertCodeBlocks(targetFormat) {
		converted = true
	}
	return converted
}

func (dr *DocRequired) ConvertCodeBlocks(targetFormat string) bool {
	if dr == nil {
		return false
	}

	converted := false
	for _, input := range dr.Inputs {
		if input.ConvertCodeBlocks(targetFormat) {
			converted = true
		}
	}
	if dr.Result.ConvertCodeBlocks(targetFormat) {
		converted = true
	}
	return converted
}

func (dc *DocConvert) ConvertCodeBlocks(targetFormat string) bool {
	if dc == nil {
		return false
	}

	converted := false
	if dc.From.ConvertCodeBlocks(targetFormat) {
		converted = true
	}
	if dc.To.ConvertCodeBlocks(targetFormat) {
		converted = true
	}
	return converted
}

func (df *DocFixit) ConvertCodeBlocks(targetFormat string) bool {
	if df == nil {
		return false
	}

	converted := false
	if df.Original.Code != "" && df.Original.ConvertCodeBlocks(targetFormat) {
		converted = true
	}
	if df.Bad.ConvertCodeBlocks(targetFormat) {
		converted = true
	}
	if df.Good.ConvertCodeBlocks(targetFormat) {
		converted = true
	}
	return converted
}

func (dc *DocCompare) ConvertCodeBlocks(targetFormat string) bool {
	if dc == nil {
		return false
	}

	converted := false
	if dc.Left.ConvertCodeBlocks(targetFormat) {
		converted = true
	}
	if dc.Right.ConvertCodeBlocks(targetFormat) {
		converted = true
	}
	if dc.Result.ConvertCodeBlocks(targetFormat) {
		converted = true
	}
	return converted
}

func (de *DocExample) ConvertCodeBlocks(targetFormat string) bool {
	if de == nil {
		return false
	}

	converted := false

	if de.Evaluate != nil {
		converted = de.Evaluate.ConvertCodeBlocks(targetFormat) || converted
	}
	if de.Diff != nil {
		converted = de.Diff.ConvertCodeBlocks(targetFormat) || converted
	}
	if de.Intersect != nil {
		converted = de.Intersect.ConvertCodeBlocks(targetFormat) || converted
	}
	if de.Required != nil {
		converted = de.Required.ConvertCodeBlocks(targetFormat) || converted
	}
	if de.Convert != nil {
		converted = de.Convert.ConvertCodeBlocks(targetFormat) || converted
	}
	if de.Fixit != nil {
		converted = de.Fixit.ConvertCodeBlocks(targetFormat) || converted
	}
	if de.Compare != nil {
		converted = de.Compare.ConvertCodeBlocks(targetFormat) || converted
	}

	return converted
}

func (ds *DocSection) ConvertCodeBlocks(targetFormat string) bool {
	if ds == nil {
		return false
	}

	converted := false

	for i := range ds.Items {
		item := &ds.Items[i]

		if item.Example != nil {
			if item.Example.ConvertCodeBlocks(targetFormat) {
				converted = true
			}
		}

		if item.Code != nil {
			if item.Code.ConvertCodeBlocks(targetFormat) {
				converted = true
			}
		}

		if item.SideBySide != nil {
			if item.SideBySide.Left.ConvertCodeBlocks(targetFormat) {
				converted = true
			}
			if item.SideBySide.Right.ConvertCodeBlocks(targetFormat) {
				converted = true
			}
		}
	}

	return converted
}

func CountKeywordMatches(text string, keywords []string) int {
	count := 0
	textLower := strings.ToLower(text)
	for _, keyword := range keywords {
		if strings.Contains(textLower, keyword) {
			count++
		}
	}
	return count
}

func (dl *DocLayer) Score(keywords []string) int {
	if dl == nil {
		return 0
	}

	score := 0
	score += CountKeywordMatches(dl.Code, keywords) * 5
	score += CountKeywordMatches(dl.Label, keywords) * 5

	return score
}

func (de *DocEvaluate) Score(keywords []string) int {
	if de == nil {
		return 0
	}

	score := 0
	for _, input := range de.Inputs {
		inputScore := input.Score(keywords)
		if inputScore > 0 {
			score += inputScore
			break
		}
	}
	score += de.Result.Score(keywords)

	return score
}

func (dd *DocDiff) Score(keywords []string) int {
	if dd == nil {
		return 0
	}

	score := 0
	score += dd.Base.Score(keywords)
	score += dd.Target.Score(keywords)
	score += dd.Result.Score(keywords)

	return score
}

func (di *DocIntersect) Score(keywords []string) int {
	if di == nil {
		return 0
	}

	score := 0
	for _, input := range di.Inputs {
		inputScore := input.Score(keywords)
		if inputScore > 0 {
			score += inputScore
			break
		}
	}
	score += di.Result.Score(keywords)

	return score
}

func (dr *DocRequired) Score(keywords []string) int {
	if dr == nil {
		return 0
	}

	score := 0
	for _, input := range dr.Inputs {
		score += input.Score(keywords)
	}
	score += dr.Result.Score(keywords)

	return score
}

func (dc *DocConvert) Score(keywords []string) int {
	if dc == nil {
		return 0
	}

	score := 0
	score += dc.From.Score(keywords)
	score += dc.To.Score(keywords)

	return score
}

func (df *DocFixit) Score(keywords []string) int {
	if df == nil {
		return 0
	}

	score := 0
	if df.Original.Code != "" {
		score += df.Original.Score(keywords)
	}
	score += df.Bad.Score(keywords)
	score += df.Good.Score(keywords)

	return score
}

func (dc *DocCompare) Score(keywords []string) int {
	if dc == nil {
		return 0
	}

	score := 0
	score += dc.Left.Score(keywords)
	score += dc.Right.Score(keywords)
	score += dc.Result.Score(keywords)

	return score
}

func (de *DocExample) Score(keywords []string) int {
	if de == nil {
		return 0
	}

	score := 0
	score += CountKeywordMatches(de.Description, keywords) * 15

	if de.Evaluate != nil {
		score += de.Evaluate.Score(keywords)
	}
	if de.Diff != nil {
		score += de.Diff.Score(keywords)
	}
	if de.Intersect != nil {
		score += de.Intersect.Score(keywords)
	}
	if de.Required != nil {
		score += de.Required.Score(keywords)
	}
	if de.Convert != nil {
		score += de.Convert.Score(keywords)
	}
	if de.Fixit != nil {
		score += de.Fixit.Score(keywords)
	}
	if de.Compare != nil {
		score += de.Compare.Score(keywords)
	}

	return score
}

func (di *DocItem) Score(keywords []string) int {
	score := 0

	if di.Content != "" {
		score += CountKeywordMatches(di.Content, keywords) * 8
	}

	if di.Example != nil {
		score += di.Example.Score(keywords)
	}

	if di.Code != nil {
		score += di.Code.Score(keywords)
	}

	if di.SideBySide != nil {
		score += di.SideBySide.Score(keywords)
	}

	return score
}

func (ds *DocSection) Score(keywords []string) int {
	if ds == nil {
		return 0
	}

	score := 0

	score += CountKeywordMatches(ds.Title, keywords) * 20
	score += CountKeywordMatches(ds.ID, keywords) * 15
	score += CountKeywordMatches(ds.Source, keywords) * 30

	for i := range ds.Items {
		score += ds.Items[i].Score(keywords)
	}

	return score
}

func (ds *DocSideBySide) Score(keywords []string) int {
	if ds == nil {
		return 0
	}

	score := 0
	score += ds.Left.Score(keywords)
	score += ds.Right.Score(keywords)

	return score
}
