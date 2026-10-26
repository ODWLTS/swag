package swag

import (
	"context"
	"errors"
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListPackages(t *testing.T) {

	cases := []struct {
		name      string
		args      []string
		searchDir string
		except    error
	}{
		{
			name:      "errorArgs",
			args:      []string{"-abc"},
			searchDir: "testdata/golist",
			except:    fmt.Errorf("exit status 2"),
		},
		{
			name:      "normal",
			args:      []string{"-deps"},
			searchDir: "testdata/golist",
			except:    nil,
		},
		{
			name:      "list error",
			args:      []string{"-deps"},
			searchDir: "testdata/golist_not_exist",
			except:    errors.New("searchDir not exist"),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := listPackages(context.TODO(), c.searchDir, nil, c.args...)
			if c.except != nil {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestGetAllGoFileInfoFromDepsByList(t *testing.T) {
	p := New(ParseUsingGoList(true))
	pwd, err := os.Getwd()
	assert.NoError(t, err)
	cases := []struct {
		name           string
		buildPackage   *build.Package
		ignoreInternal bool
		except         error
	}{
		{
			name: "normal",
			buildPackage: &build.Package{
				Name:       "main",
				ImportPath: "github.com/ODWLTS/swag/testdata/golist",
				Dir:        "testdata/golist",
				GoFiles:    []string{"main.go"},
				CgoFiles:   []string{"api/api.go"},
			},
			except: nil,
		},
		{
			name: "ignore internal",
			buildPackage: &build.Package{
				Goroot: true,
			},
			ignoreInternal: true,
			except:         nil,
		},
		{
			name: "gofiles error",
			buildPackage: &build.Package{
				Dir:     "testdata/golist_not_exist",
				GoFiles: []string{"main.go"},
			},
			except: errors.New("file not exist"),
		},
		{
			name: "cgofiles error",
			buildPackage: &build.Package{
				Dir:      "testdata/golist_not_exist",
				CgoFiles: []string{"main.go"},
			},
			except: errors.New("file not exist"),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.ignoreInternal {
				p.ParseInternal = false
			}
			c.buildPackage.Dir = filepath.Join(pwd, c.buildPackage.Dir)
			err := p.getAllGoFileInfoFromDepsByList(c.buildPackage)
			if c.except != nil {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

const (
	lazyDependencySearchDir = "testdata/lazy_dependency/main"
	lazyExternalPkg         = "github.com/ODWLTS/swag/testdata/lazy_dependency/external"
	lazyNestedPkg           = "github.com/ODWLTS/swag/testdata/lazy_dependency/external/nested"
)

// The handler in testdata/lazy_dependency/api references external.Thing, whose
// package is not in main's dependency set, so it is loaded lazily.
func TestParseLazyDependencyByGoList(t *testing.T) {
	p := New(ParseUsingGoList(true))
	p.ParseDependency = true

	err := p.ParseAPI(lazyDependencySearchDir, "main.go", defaultParseDepth)
	assert.NoError(t, err)

	assert.Contains(t, p.swagger.Definitions, "external.Thing")
	assert.Contains(t, p.swagger.Definitions, "nested.Detail")

	// the go list path collects files, the go/loader path never does
	assert.True(t, p.packages.hasCollectedFiles(lazyExternalPkg))
	assert.True(t, p.packages.hasCollectedFiles(lazyNestedPkg))
}

func TestParseLazyDependencyFallsBackToLoader(t *testing.T) {
	p := New(ParseUsingGoList(true))
	p.ParseDependency = true
	p.packages.loadDependency = func(string) (bool, error) { return false, nil }

	err := p.ParseAPI(lazyDependencySearchDir, "main.go", defaultParseDepth)
	assert.NoError(t, err)

	assert.Contains(t, p.swagger.Definitions, "external.Thing")
	assert.Contains(t, p.swagger.Definitions, "nested.Detail")
	assert.False(t, p.packages.hasCollectedFiles(lazyExternalPkg))
}

func TestLoadDependencyByList(t *testing.T) {
	t.Run("loads the package and its dependencies once", func(t *testing.T) {
		p := New()

		loaded, err := p.loadDependencyByList(lazyExternalPkg)
		assert.NoError(t, err)
		assert.True(t, loaded)
		assert.True(t, p.packages.hasCollectedFiles(lazyExternalPkg))
		assert.True(t, p.packages.hasCollectedFiles(lazyNestedPkg))
		assert.NotNil(t, p.packages.findTypeSpec(lazyExternalPkg, "Thing"))
		assert.NotNil(t, p.packages.findTypeSpec(lazyNestedPkg, "Detail"))

		filesBefore := len(p.packages.files)
		loaded, err = p.loadDependencyByList(lazyExternalPkg)
		assert.NoError(t, err)
		assert.True(t, loaded)
		assert.Equal(t, filesBefore, len(p.packages.files), "already collected packages are not parsed again")
	})

	t.Run("excluded dependencies stay out", func(t *testing.T) {
		p := New(SetExcludeDependencies("nested"))

		loaded, err := p.loadDependencyByList(lazyExternalPkg)
		assert.NoError(t, err)
		assert.True(t, loaded)
		assert.True(t, p.packages.hasCollectedFiles(lazyExternalPkg))
		assert.False(t, p.packages.hasCollectedFiles(lazyNestedPkg))
	})

	t.Run("reports not loaded for an unknown import path", func(t *testing.T) {
		p := New()

		loaded, err := p.loadDependencyByList("github.com/ODWLTS/swag/testdata/lazy_dependency/missing")
		assert.NoError(t, err)
		assert.False(t, loaded)
	})

	t.Run("reports not loaded outside of a module", func(t *testing.T) {
		wd, err := os.Getwd()
		assert.NoError(t, err)
		defer func() { assert.NoError(t, os.Chdir(wd)) }()

		// outside of any module, go list cannot resolve the import path
		assert.NoError(t, os.Chdir(t.TempDir()))
		go111module, hadEnv := os.LookupEnv("GO111MODULE")
		assert.NoError(t, os.Setenv("GO111MODULE", "on"))
		defer func() {
			if hadEnv {
				_ = os.Setenv("GO111MODULE", go111module)
			} else {
				_ = os.Unsetenv("GO111MODULE")
			}
		}()

		p := New()
		loaded, err := p.loadDependencyByList(lazyExternalPkg)
		assert.NoError(t, err)
		assert.False(t, loaded)
	})
}
