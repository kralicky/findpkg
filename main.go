package main

import (
	"regexp"
	"sort"
	"strings"

	"github.com/jedib0t/go-pretty/v6/list"
	flag "github.com/spf13/pflag"
	"github.com/ttacon/chalk"
	"golang.org/x/tools/go/packages"
)

func main() {
	var pattern string
	var tags []string
	var tests bool
	flag.StringVar(&pattern, "pattern", "", "Regex pattern to match against package paths")
	flag.StringSliceVar(&tags, "tags", []string{}, "Build tags to use")
	flag.BoolVar(&tests, "tests", true, "Include test packages")
	flag.Parse()

	var buildFlags []string
	if len(tags) > 0 {
		buildFlags = append(buildFlags, "-tags", strings.Join(tags, ","))
	}
	topLevelPackages := []string{"."}
	if args := flag.Args(); len(args) > 0 {
		topLevelPackages = args
	}

	patternRegex := regexp.MustCompile(pattern)

	allPackages, err := packages.Load(&packages.Config{
		Mode:       packages.NeedName | packages.NeedFiles | packages.NeedImports | packages.NeedDeps,
		Tests:      tests,
		BuildFlags: buildFlags,
	}, topLevelPackages...)
	if err != nil {
		panic(err)
	}

	var visit func(*packages.Package) (branches [][]*packages.Package)
	seen := make(map[*packages.Package]bool)
	visit = func(pkg *packages.Package) [][]*packages.Package {
		if seen[pkg] {
			return nil
		}
		seen[pkg] = true

		var branches [][]*packages.Package

		if patternRegex.Match([]byte(pkg.PkgPath)) {
			return append(branches, []*packages.Package{pkg})
		}

		imports := make([]string, 0, len(pkg.Imports))
		for name := range pkg.Imports {
			imports = append(imports, name)
		}
		sort.Strings(imports)

		for _, imp := range imports {
			branches = append(branches, visit(pkg.Imports[imp])...)
		}
		for i, branch := range branches {
			if len(branch) > 0 {
				branches[i] = append([]*packages.Package{pkg}, branch...)
			}
		}

		return branches
	}
	for _, pkg := range allPackages {
		branches := visit(pkg)
		if len(branches) == 0 {
			continue
		}
		tree := mergeBranches(branches)
		tree.pkg = pkg
		lw := list.NewWriter()
		lw.SetStyle(list.StyleConnectedLight)
		buildTree(tree, lw)
		println(lw.Render())
	}
}

type treeNode struct {
	pkg     *packages.Package
	imports []*treeNode
}

func mergeBranches(branches [][]*packages.Package) *treeNode {
	root := &treeNode{}

	for _, branch := range branches {
		currentNode := root
		for _, pkg := range branch {
			found := false
			for _, child := range currentNode.imports {
				if child.pkg == pkg {
					currentNode = child
					found = true
					break
				}
			}
			if !found {
				newNode := &treeNode{pkg: pkg}
				currentNode.imports = append(currentNode.imports, newNode)
				currentNode = newNode
			}
		}
	}

	return root
}

func buildTree(node *treeNode, lw list.Writer) {
	name := node.pkg.PkgPath
	if len(node.imports) == 0 {
		name = chalk.Red.Color(name)
	}
	lw.AppendItem(name)
	for _, child := range node.imports {
		lw.Indent()
		buildTree(child, lw)
		lw.UnIndent()
	}
}
