// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

package yamlgen

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/dave/jennifer/jen"
	"github.com/pkg/errors"
)

// TODO(saswatamcode): Add tests.
// TODO(saswatamcode): Check jennifer code for some safety.
// TODO(saswatamcode): Add mechanism for caching output from generated code.
// TODO(saswatamcode): Currently takes file names, need to make it module based(something such as https://golang.org/pkg/cmd/go/internal/list/).

// GenGoCode generates Go code for yaml gen from structs in src file.
func GenGoCode(src []byte) (string, error) {
	// Create new main file.
	fset := token.NewFileSet()
	generatedCode := jen.NewFile("main")

	// Parse source file.
	f, err := parser.ParseFile(fset, "", src, parser.AllErrors)
	if err != nil {
		return "", err
	}

	// Add imports if needed(will not be used if not required in code).
	for _, s := range f.Imports {
		generatedCode.ImportName(s.Path.Value[1:len(s.Path.Value)-1], "")
	}

	// Init statements for structs.
	var init []jen.Code
	// Declare config map, i.e, `configs := map[string]interface{}{}`.
	init = append(init, jen.Id("configs").Op(":=").Map(jen.String()).Interface().Values())

	// Loop through declarations in file.
	for _, decl := range f.Decls {
		// Cast to generic declaration node.
		if genericDecl, ok := decl.(*ast.GenDecl); ok {
			// Check if declaration spec is `type`.
			if typeDecl, ok := genericDecl.Specs[0].(*ast.TypeSpec); ok {
				var structFields []jen.Code
				// Cast to `type struct`.
				structDecl, ok := typeDecl.Type.(*ast.StructType)
				if !ok {
					generatedCode.Type().Id(typeDecl.Name.Name).Id(string(src[typeDecl.Type.Pos()-1 : typeDecl.Type.End()-1]))
					continue
				}
				fields := structDecl.Fields.List
				arrayInit := make(jen.Dict)

				// Loop and generate fields for each field.
				for _, field := range fields {
					// Each field might have multiple names.
					names := field.Names
					for _, n := range names {
						if n.IsExported() {
							pos := n.Obj.Decl.(*ast.Field)

							// Check if field is a slice type.
							sliceRe := regexp.MustCompile(`.*\[.*\].*`)
							if sliceRe.MatchString(types.ExprString(field.Type)) {
								arrayInit[jen.Id(n.Name)] = jen.Id(string(src[pos.Type.Pos()-1 : pos.Type.End()-1])).Values(jen.Id(string(src[pos.Type.Pos()+1 : pos.Type.End()-1])).Values())
							}

							// Copy struct field to generated code.
							if pos.Tag != nil {
								structFields = append(structFields, jen.Id(n.Name).Id(string(src[pos.Type.Pos()-1:pos.Type.End()-1])).Id(pos.Tag.Value))
							}
						}
					}
				}

				// Add initialize statements for struct via code like `configs["Type"] = Type{}`.
				// If struct has array members, use array initializer via code like `configs["Config"] = Config{ArrayMember: []Type{Type{}}}`.
				init = append(init, jen.Id("configs").Index(jen.Lit(typeDecl.Name.Name)).Op("=").Id(typeDecl.Name.Name).Values(arrayInit))

				// Finally put struct inside generated code.
				generatedCode.Type().Id(typeDecl.Name.Name).Struct(structFields...)
			}
		}
	}

	// Add for loop to iterate through map and return config YAML.
	init = append(init, jen.For(
		jen.List(jen.Id("k"), jen.Id("config")).Op(":=").Range().Id("configs"),
	).Block(
		// We import the cfggen Generate method directly to generate output.
		jen.Qual("fmt", "Println").Call(jen.Lit("---")),
		jen.Qual("fmt", "Println").Call(jen.Id("k")),
		// TODO(saswatamcode): Replace with import from mdox itself once merged.
		jen.Qual("github.com/bwplotka/mdox/pkg/yamlgen", "Generate").Call(jen.Id("config"), jen.Qual("os", "Stderr")),
	))

	// Generate main function in new module.
	generatedCode.Func().Id("main").Params().Block(init...)
	return fmt.Sprintf("%#v", generatedCode), nil
}

// execGoCode executes and returns output from generated Go code.
func ExecGoCode(ctx context.Context, mainGo string) ([]byte, error) {
	tmpDir, err := ioutil.TempDir("", "structgen")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	// Copy generated code to main.go.
	main, err := os.Create(filepath.Join(tmpDir, "main.go"))
	if err != nil {
		return nil, err
	}
	defer main.Close()

	_, err = main.Write([]byte(mainGo))
	if err != nil {
		return nil, err
	}

	// Create go.mod in temp dir.
	cmd := exec.CommandContext(ctx, "go", "mod", "init", "structgen")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		return nil, errors.Wrapf(err, "mod init %v", cmd)
	}

	// Replace for unreleased mdox yamlgen so don't need to copy cfggen code to new dir and compile.
	// Currently in github.com/saswatamcode/mdox@v0.2.2-0.20210823074517-0245f9afb0a8. Replace once #79 is merged.
	cmd = exec.CommandContext(ctx, "go", "mod", "edit", "-replace", "github.com/bwplotka/mdox=github.com/saswatamcode/mdox@v0.2.2-0.20210823074517-0245f9afb0a8")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		return nil, errors.Wrapf(err, "mod edit %v", cmd)
	}

	// Import required packages(generate go.sum).
	cmd = exec.CommandContext(ctx, "go", "mod", "tidy")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		return nil, errors.Wrapf(err, "mod tidy %v", cmd)
	}

	// Execute generate code and return output.
	b := bytes.Buffer{}
	cmd = exec.CommandContext(ctx, "go", "run", "main.go")
	cmd.Dir = tmpDir
	cmd.Stderr = &b
	cmd.Stdout = &b
	if err := cmd.Run(); err != nil {
		return nil, errors.Wrapf(err, "run %v out %v", cmd, b.String())
	}

	return b.Bytes(), nil
}
