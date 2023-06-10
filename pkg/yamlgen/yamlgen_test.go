// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package yamlgen

import (
	"testing"

	"github.com/efficientgo/core/testutil"
	"golang.org/x/net/context"
)

func TestYAMLGen_GenGoCode(t *testing.T) {
	t.Run("normal struct", func(t *testing.T) {
		source := []byte("package main\n\ntype TestConfig struct {\n\tUrl   string `yaml:\"url\"`\n\tID    int    `yaml:\"id\"`\n\tToken string `yaml:\"token\"`\n}\n")
		generatedCode, err := GenGoCode(source)
		testutil.Ok(t, err)

		expected := "package main\n\nimport (\n\t\"fmt\"\n\tyamlgen \"github.com/bwplotka/mdox/pkg/yamlgen\"\n\t\"os\"\n)\n\ntype TestConfig struct {\n\tUrl   string `yaml:\"url\"`\n\tID    int    `yaml:\"id\"`\n\tToken string `yaml:\"token\"`\n}\n\nfunc main() {\n\tconfigs := map[string]interface{}{}\n\tconfigs[\"TestConfig\"] = TestConfig{}\n\tfor k, config := range configs {\n\t\tfmt.Println(\"---\")\n\t\tfmt.Println(k)\n\t\tyamlgen.Generate(config, os.Stderr)\n\t}\n}\n"
		testutil.Equals(t, expected, generatedCode)
	})

	t.Run("struct with unexported field", func(t *testing.T) {
		source := []byte("package main\n\nimport \"regexp\"\n\ntype ValidatorConfig struct {\n\tType  string `yaml:\"type\"`\n\tRegex string `yaml:\"regex\"`\n\tToken string `yaml:\"token\"`\n\n\tr *regexp.Regexp\n}\n")
		generatedCode, err := GenGoCode(source)
		testutil.Ok(t, err)

		expected := "package main\n\nimport (\n\t\"fmt\"\n\tyamlgen \"github.com/bwplotka/mdox/pkg/yamlgen\"\n\t\"os\"\n)\n\ntype ValidatorConfig struct {\n\tType  string `yaml:\"type\"`\n\tRegex string `yaml:\"regex\"`\n\tToken string `yaml:\"token\"`\n}\n\nfunc main() {\n\tconfigs := map[string]interface{}{}\n\tconfigs[\"ValidatorConfig\"] = ValidatorConfig{}\n\tfor k, config := range configs {\n\t\tfmt.Println(\"---\")\n\t\tfmt.Println(k)\n\t\tyamlgen.Generate(config, os.Stderr)\n\t}\n}\n"
		testutil.Equals(t, expected, generatedCode)
	})

	t.Run("struct with array fields", func(t *testing.T) {
		source := []byte("package main\n\nimport \"regexp\"\n\ntype Config struct {\n\tVersion int `yaml:\"version\"`\n\n\tValidator []ValidatorConfig `yaml:\"validators\"`\n\tIgnore    []IgnoreConfig    `yaml:\"ignore\"`\n}\n\ntype ValidatorConfig struct {\n\tType  string `yaml:\"type\"`\n\tRegex string `yaml:\"regex\"`\n\tToken string `yaml:\"token\"`\n\n\tr *regexp.Regexp\n}\n\ntype IgnoreConfig struct {\n\tUrl   string `yaml:\"url\"`\n\tID    int    `yaml:\"id\"`\n\tToken string `yaml:\"token\"`\n}\n")
		generatedCode, err := GenGoCode(source)
		testutil.Ok(t, err)

		expected := "package main\n\nimport (\n\t\"fmt\"\n\tyamlgen \"github.com/bwplotka/mdox/pkg/yamlgen\"\n\t\"os\"\n)\n\ntype Config struct {\n\tVersion   int               `yaml:\"version\"`\n\tValidator []ValidatorConfig `yaml:\"validators\"`\n\tIgnore    []IgnoreConfig    `yaml:\"ignore\"`\n}\ntype ValidatorConfig struct {\n\tType  string `yaml:\"type\"`\n\tRegex string `yaml:\"regex\"`\n\tToken string `yaml:\"token\"`\n}\ntype IgnoreConfig struct {\n\tUrl   string `yaml:\"url\"`\n\tID    int    `yaml:\"id\"`\n\tToken string `yaml:\"token\"`\n}\n\nfunc main() {\n\tconfigs := map[string]interface{}{}\n\tconfigs[\"Config\"] = Config{\n\t\tIgnore:    []IgnoreConfig{IgnoreConfig{}},\n\t\tValidator: []ValidatorConfig{ValidatorConfig{}},\n\t}\n\tconfigs[\"ValidatorConfig\"] = ValidatorConfig{}\n\tconfigs[\"IgnoreConfig\"] = IgnoreConfig{}\n\tfor k, config := range configs {\n\t\tfmt.Println(\"---\")\n\t\tfmt.Println(k)\n\t\tyamlgen.Generate(config, os.Stderr)\n\t}\n}\n"
		testutil.Equals(t, expected, generatedCode)
	})
}

func TestYAMLGen_ExecGoCode(t *testing.T) {
	t.Run("normal struct", func(t *testing.T) {
		generatedCode := "package main\n\nimport (\n\t\"fmt\"\n\tyamlgen \"github.com/bwplotka/mdox/pkg/yamlgen\"\n\t\"os\"\n)\n\ntype TestConfig struct {\n\tUrl   string `yaml:\"url\"`\n\tID    int    `yaml:\"id\"`\n\tToken string `yaml:\"token\"`\n}\n\nfunc main() {\n\tconfigs := map[string]interface{}{}\n\tconfigs[\"TestConfig\"] = TestConfig{}\n\tfor k, config := range configs {\n\t\tfmt.Println(\"---\")\n\t\tfmt.Println(k)\n\t\tyamlgen.Generate(config, os.Stderr)\n\t}\n}\n"
		output, err := ExecGoCode(context.TODO(), generatedCode)
		testutil.Ok(t, err)

		expected := "---\nTestConfig\nurl: \"\"\nid: 0\ntoken: \"\"\n"
		testutil.Equals(t, expected, string(output))
	})

	t.Run("struct with array fields", func(t *testing.T) {
		generatedCode := "package main\n\nimport (\n\t\"fmt\"\n\tyamlgen \"github.com/bwplotka/mdox/pkg/yamlgen\"\n\t\"os\"\n)\n\ntype Config struct {\n\tVersion   int               `yaml:\"version\"`\n\tValidator []ValidatorConfig `yaml:\"validators\"`\n\tIgnore    []IgnoreConfig    `yaml:\"ignore\"`\n}\ntype ValidatorConfig struct {\n\tType  string `yaml:\"type\"`\n\tRegex string `yaml:\"regex\"`\n\tToken string `yaml:\"token\"`\n}\ntype IgnoreConfig struct {\n\tUrl   string `yaml:\"url\"`\n\tID    int    `yaml:\"id\"`\n\tToken string `yaml:\"token\"`\n}\n\nfunc main() {\n\tconfigs := map[string]interface{}{}\n\tconfigs[\"Config\"] = Config{\n\t\tIgnore:    []IgnoreConfig{IgnoreConfig{}},\n\t\tValidator: []ValidatorConfig{ValidatorConfig{}},\n\t}\n\tconfigs[\"ValidatorConfig\"] = ValidatorConfig{}\n\tconfigs[\"IgnoreConfig\"] = IgnoreConfig{}\n\tfor k, config := range configs {\n\t\tfmt.Println(\"---\")\n\t\tfmt.Println(k)\n\t\tyamlgen.Generate(config, os.Stderr)\n\t}\n}\n"
		output, err := ExecGoCode(context.TODO(), generatedCode)
		testutil.Ok(t, err)

		expected := "---\nConfig\nversion: 0\nvalidators:\n    - type: \"\"\n      regex: \"\"\n      token: \"\"\nignore:\n    - url: \"\"\n      id: 0\n      token: \"\"\n---\nValidatorConfig\ntype: \"\"\nregex: \"\"\ntoken: \"\"\n---\nIgnoreConfig\nurl: \"\"\nid: 0\ntoken: \"\"\n"
		testutil.Equals(t, expected, string(output))
	})
}
