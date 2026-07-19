//go:build ignore

// apply_separate_package injects the separate_package flag into protoc-gen-go-grpc
// sources. A single git patch cannot cover all released layouts (v1.2–v1.6).
package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	if err := patchMain("cmd/protoc-gen-go-grpc/main.go"); err != nil {
		fmt.Fprintf(os.Stderr, "main.go: %v\n", err)
		os.Exit(1)
	}
	if err := patchGRPC("cmd/protoc-gen-go-grpc/grpc.go"); err != nil {
		fmt.Fprintf(os.Stderr, "grpc.go: %v\n", err)
		os.Exit(1)
	}
}

func patchMain(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	src := string(data)
	if strings.Contains(src, "separatePackage") {
		return nil
	}

	decl := "var requireUnimplemented *bool\n"
	if !strings.Contains(src, decl) {
		return fmt.Errorf("requireUnimplemented decl not found")
	}
	src = strings.Replace(src, decl, decl+"var separatePackage *bool\n", 1)

	flag := "requireUnimplemented = flags.Bool(\"require_unimplemented_servers\", true, \"set to false to match legacy behavior\")\n"
	if !strings.Contains(src, flag) {
		return fmt.Errorf("requireUnimplemented flag not found")
	}
	src = strings.Replace(src, flag, flag+
		"\tseparatePackage = flags.Bool(\"separate_package\", false, \"set to true to write generated files to a separate grpc package\")\n", 1)

	return os.WriteFile(path, []byte(src), 0o644)
}

func patchGRPC(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	src := string(data)
	if strings.Contains(src, "separatePackage") {
		return nil
	}

	if !strings.Contains(src, "\t\"path\"\n") {
		if !strings.Contains(src, "\t\"fmt\"\n") {
			return fmt.Errorf("fmt import not found")
		}
		src = strings.Replace(src, "\t\"fmt\"\n", "\t\"fmt\"\n\t\"path\"\n", 1)
	}

	old := "\tfilename := file.GeneratedFilenamePrefix + \"_grpc.pb.go\"\n" +
		"\tg := gen.NewGeneratedFile(filename, file.GoImportPath)\n"
	if !strings.Contains(src, old) {
		return fmt.Errorf("filename assignment not found")
	}
	new := "\tvar g *protogen.GeneratedFile\n" +
		"\tif !*separatePackage {\n" +
		"\t\tfilename := file.GeneratedFilenamePrefix + \"_grpc.pb.go\"\n" +
		"\t\tg = gen.NewGeneratedFile(filename, file.GoImportPath)\n" +
		"\t} else {\n" +
		"\t\tfile.GoPackageName += \"grpc\"\n" +
		"\t\tdir := path.Dir(file.GeneratedFilenamePrefix)\n" +
		"\t\tbase := path.Base(file.GeneratedFilenamePrefix)\n" +
		"\t\tfile.GeneratedFilenamePrefix = path.Join(\n" +
		"\t\t\tdir,\n" +
		"\t\t\tstring(file.GoPackageName),\n" +
		"\t\t\tbase,\n" +
		"\t\t)\n" +
		"\t\tg = gen.NewGeneratedFile(\n" +
		"\t\t\tfile.GeneratedFilenamePrefix+\"_grpc.pb.go\",\n" +
		"\t\t\tprotogen.GoImportPath(path.Join(\n" +
		"\t\t\t\tstring(file.GoImportPath),\n" +
		"\t\t\t\tstring(file.GoPackageName),\n" +
		"\t\t\t)),\n" +
		"\t\t)\n" +
		"\t}\n"
	src = strings.Replace(src, old, new, 1)

	return os.WriteFile(path, []byte(src), 0o644)
}
