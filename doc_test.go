package bkl_test

import (
	"fmt"
	"testing"

	"github.com/gopatchy/bkl"
)

func TestDocumentationExamples(t *testing.T) {
	t.Parallel()

	sections, err := bkl.GetDocSections()
	if err != nil {
		t.Fatalf("Failed to get doc sections: %v", err)
	}

	tests := make(map[string]*bkl.DocExample)

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

	RunTestLoop(t, tests)
}
