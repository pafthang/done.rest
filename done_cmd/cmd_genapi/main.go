package main

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/hiveot/hub/done_tool/utils"
	"gopkg.in/yaml.v3"
)

const golangFile = "./done_api/api_go/ht-vocab.go"
const jsFile = "./done_api/api_js/ht-vocab.js"
const pyFile = "./done_api/api_py/ht-vocab.py"
const classDir = "./done_api/api_vocab"

const ActionClassFile = "ht-action-classes.yaml"

// Generate the API source files from the vocabulary classes.
func main() {
	classes, err := LoadVocabFiles(classDir)
	if err == nil {
		lines := ExportToGolang(classes)
		data := strings.Join(lines, "\n")
		err = os.WriteFile(golangFile, []byte(data), 0664)
	}
	if err == nil {
		lines := ExportToJavascript(classes)
		data := strings.Join(lines, "\n")
		err = os.WriteFile(jsFile, []byte(data), 0664)
	}
	if err == nil {
		lines := ExportToPython(classes)
		data := strings.Join(lines, "\n")
		err = os.WriteFile(pyFile, []byte(data), 0664)
	}
	if err != nil {
		fmt.Println("ERROR: " + err.Error())
	}
}

// VocabClass holds a vocabulary entry
type VocabClass struct {
	ClassName   string `yaml:"class"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Symbol      string `yaml:"symbol,omitempty"` // for units
}

// VocabClassMap class map by vocabulary keyword
type VocabClassMap struct {
	Version   string                `yaml:"version"`
	Link      string                `yaml:"link"`
	Namespace string                `yaml:"namespace"`
	Vocab     map[string]VocabClass `yaml:"vocab"`
}

// LoadVocabFiles loads the thing, property and action classes
func LoadVocabFiles(dir string) (map[string]VocabClassMap, error) {
	classes := make(map[string]VocabClassMap)

	files, err := os.ReadDir(dir)
	if err != nil {
		return classes, err
	}
	for _, entry := range files {
		if strings.HasSuffix(entry.Name(), ".yaml") {
			vocabFile := path.Join(dir, entry.Name())
			data, err := os.ReadFile(vocabFile)
			if err == nil {
				yaml.Unmarshal(data, &classes)

			}
		}
	}
	return classes, err
}

// ExportToGolang writes the thing, property, action and unit classes in a golang format
func ExportToGolang(vc map[string]VocabClassMap) []string {
	lines := make([]string, 0)

	lines = append(lines, "// Package vocab with HiveOT vocabulary names for TD Things, properties, events and actions")
	lines = append(lines, "package vocab")

	// Loop through the types of vocabularies
	for classType, cm := range vc {
		vocabKeys := utils.OrderedMapKeys(cm.Vocab)

		//- export the constants
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("// type: %s", classType))
		lines = append(lines, fmt.Sprintf("// version: %s", cm.Version))
		lines = append(lines, fmt.Sprintf("// generated: %s", time.Now().Format(time.RFC822)))
		lines = append(lines, fmt.Sprintf("// source: %s", cm.Link))
		lines = append(lines, fmt.Sprintf("// namespace: %s", cm.Namespace))
		lines = append(lines, "const (")
		for _, key := range vocabKeys {
			classInfo := cm.Vocab[key]
			lines = append(lines, fmt.Sprintf("  %s = \"%s\"", key, classInfo.ClassName))
		}
		lines = append(lines, ")")
		lines = append(lines, "// end of "+classType)

		//- export the map with title and description
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("// %sMap maps @type to symbol, title and description", classType))
		lines = append(lines, fmt.Sprintf("var %sMap = map[string]struct {", classType))
		lines = append(lines, "   Symbol string; Title string; Description string")
		lines = append(lines, "} {")
		for key, unitInfo := range cm.Vocab {
			lines = append(lines, fmt.Sprintf(
				"  %s: {Symbol: \"%s\", Title: \"%s\", Description: \"%s\"},",
				key, unitInfo.Symbol, unitInfo.Title, unitInfo.Description))
		}
		lines = append(lines, "}")
		lines = append(lines, "")
	}

	return lines
}

// ExportToJavascript writes the thing, property and action classes in javascript format
func ExportToJavascript(vc map[string]VocabClassMap) []string {
	lines := make([]string, 0)

	lines = append(lines, "// Package vocab with HiveOT vocabulary names for TD Things, properties, events and actions")
	lines = append(lines, fmt.Sprintf("// DO NOT EDIT. This file is generated and changes will be overwritten"))
	// Loop through the types of vocabularies
	for classType, cm := range vc {
		vocabKeys := utils.OrderedMapKeys(cm.Vocab)

		//- export the constants
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("// type: %s", classType))
		lines = append(lines, fmt.Sprintf("// version: %s", cm.Version))
		lines = append(lines, fmt.Sprintf("// generated: %s", time.Now().Format(time.RFC822)))
		lines = append(lines, fmt.Sprintf("// source: %s", cm.Link))
		lines = append(lines, fmt.Sprintf("// namespace: %s", cm.Namespace))
		for _, key := range vocabKeys {
			classInfo := cm.Vocab[key]
			lines = append(lines, fmt.Sprintf("export const %s = \"%s\";", key, classInfo.ClassName))
		}
		lines = append(lines, "// end of "+classType)

		//- export the map with title and description
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("// %sMap maps @type to symbol, title and description", classType))
		lines = append(lines, fmt.Sprintf("export const %sMap = {", classType))
		for key, unitInfo := range cm.Vocab {
			atType := cm.Vocab[key].ClassName //
			lines = append(lines, fmt.Sprintf(
				"  \"%s\": {Symbol: \"%s\", Title: \"%s\", Description: \"%s\"},",
				atType, unitInfo.Symbol, unitInfo.Title, unitInfo.Description))
		}
		lines = append(lines, "}")
		lines = append(lines, "")
	}
	return lines
}

// ExportToPython writes the thing, property and action classes in a python format
func ExportToPython(vc map[string]VocabClassMap) []string {
	lines := make([]string, 0)

	lines = append(lines, "# Package vocab with HiveOT vocabulary names for TD Things, properties, events and actions")
	lines = append(lines, fmt.Sprintf("# DO NOT EDIT. This file is generated and changes will be overwritten"))

	// Loop through the types of vocabularies
	for classType, cm := range vc {
		vocabKeys := utils.OrderedMapKeys(cm.Vocab)

		//- export the constants
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("# type: %s", classType))
		lines = append(lines, fmt.Sprintf("# version: %s", cm.Version))
		lines = append(lines, fmt.Sprintf("# generated: %s", time.Now().Format(time.RFC822)))
		lines = append(lines, fmt.Sprintf("# source: %s", cm.Link))
		lines = append(lines, fmt.Sprintf("# namespace: %s", cm.Namespace))
		for _, key := range vocabKeys {
			classInfo := cm.Vocab[key]
			lines = append(lines, fmt.Sprintf("%s = \"%s\"", key, classInfo.ClassName))
		}
		lines = append(lines, "# end of "+classType)

		//- export the map with title and description
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("# %sMap maps @type to symbol, title and description", classType))
		lines = append(lines, fmt.Sprintf("%sMap = {", classType))
		for key, unitInfo := range cm.Vocab {
			atType := cm.Vocab[key].ClassName //
			lines = append(lines, fmt.Sprintf(
				"  \"%s\": {\"Symbol\": \"%s\", \"Title\": \"%s\", \"Description\": \"%s\"},",
				atType, unitInfo.Symbol, unitInfo.Title, unitInfo.Description))
		}
		lines = append(lines, "}")
		lines = append(lines, "")
	}
	return lines
}
