package swag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/build"
	"os"
	"os/exec"
	"path/filepath"
)

func listPackages(ctx context.Context, dir string, env []string, args ...string) (pkgs []*build.Package, finalErr error) {
	cmd := exec.CommandContext(ctx, "go", append([]string{"list", "-json", "-e"}, args...)...)
	cmd.Env = env
	cmd.Dir = dir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf
	defer func() {
		if stderrBuf.Len() > 0 {
			finalErr = fmt.Errorf("%v\n%s", finalErr, stderrBuf.Bytes())
		}
	}()

	err = cmd.Start()
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(stdout)
	for dec.More() {
		var pkg build.Package
		err = dec.Decode(&pkg)
		if err != nil {
			return nil, err
		}
		pkgs = append(pkgs, &pkg)
	}
	err = cmd.Wait()
	if err != nil {
		return nil, err
	}
	return pkgs, nil
}

func (parser *Parser) getAllGoFileInfoFromDepsByList(pkg *build.Package) error {
	ignoreInternal := pkg.Goroot && !parser.ParseInternal
	if ignoreInternal { // ignored internal
		return nil
	}

	return parser.collectGoFilesOfPackage(pkg)
}

// collectGoFilesOfPackage parses the Go files of a package listed by `go list`
// and collects them, unless the package is excluded with --excludeDependencies.
func (parser *Parser) collectGoFilesOfPackage(pkg *build.Package) error {
	if len(parser.excludeDependencies) > 0 {
		if _, ok := parser.excludeDependencies[pkg.Name]; ok {
			return nil
		}
	}

	srcDir := pkg.Dir
	var err error
	for i := range pkg.GoFiles {
		err = parser.parseFile(pkg.ImportPath, filepath.Join(srcDir, pkg.GoFiles[i]), nil)
		if err != nil {
			return err
		}
	}

	// parse .go source files that import "C"
	for i := range pkg.CgoFiles {
		err = parser.parseFile(pkg.ImportPath, filepath.Join(srcDir, pkg.CgoFiles[i]), nil)
		if err != nil {
			return err
		}
	}

	return nil
}

// loadDependencyByList loads the package at importPath and its transitive
// dependencies with `go list -deps`, parsing only the Go files of packages
// that have not been collected yet; nothing is type-checked. Like the
// go/loader based loader it replaces, it does not skip GOROOT packages when
// --parseInternal is off, so a lazily resolved std type still gets a schema.
//
// It reports false, without an error, when `go list` cannot resolve the import
// path so that the caller can try another loader.
func (parser *Parser) loadDependencyByList(importPath string) (bool, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return false, err
	}

	pkgs, err := listPackages(context.Background(), cwd, nil, "-deps", importPath)
	if err != nil {
		parser.debug.Printf("warning: go list failed for dependency %s, falling back to go/loader: %v", importPath, err)

		return false, nil
	}

	// with -e an unresolvable package is still listed, without a directory
	resolved := false
	for _, pkg := range pkgs {
		if pkg.ImportPath == importPath && pkg.Dir != "" {
			resolved = true

			break
		}
	}

	if !resolved {
		parser.debug.Printf("warning: go list could not resolve dependency %s, falling back to go/loader", importPath)

		return false, nil
	}

	for _, pkg := range pkgs {
		if parser.packages.hasCollectedFiles(pkg.ImportPath) {
			continue
		}

		err = parser.collectGoFilesOfPackage(pkg)
		if err != nil {
			return true, err
		}

		parser.packages.parseTypesFromPackage(pkg.ImportPath)
	}

	return true, nil
}
