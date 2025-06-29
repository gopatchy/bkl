package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"
	"strings"

	"github.com/gopatchy/bkl"
	"gopkg.in/yaml.v3"
)

// TemplateData holds all data for the template
type TemplateData struct {
	IntroText1 string
	IntroText2 string
	Badges     string
	Sections   []bkl.DocSection
}

func main() {
	data, err := os.ReadFile("sections.yaml")
	if err != nil {
		log.Fatal(err)
	}

	var sections []bkl.DocSection
	if err := yaml.Unmarshal(data, &sections); err != nil {
		log.Fatal(err)
	}

	templateContent, err := os.ReadFile("template.html")
	if err != nil {
		log.Fatal(err)
	}

	tmpl := template.Must(template.New("html").Funcs(template.FuncMap{
		"formatContent": formatContent,
		"formatLayer":   formatLayer,
	}).Parse(string(templateContent)))

	templateData := TemplateData{
		Sections: sections,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData); err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile("index.html", buf.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Generated index.html")
}

func formatContent(content string) template.HTML {
	return template.HTML(content)
}

func formatLayer(layer bkl.DocLayer) template.HTML {
	code := strings.TrimSpace(layer.Code)

	// Apply syntax highlighting with highlights
	result := applySyntaxHighlighting(code, layer.Languages, layer.Highlights)

	return template.HTML(result)
}
