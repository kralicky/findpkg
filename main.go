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
		Mode:       packages.NeedName | packages.NeedFiles | packages.NeedImports | packages.NeedDeps | packages.NeedModule,
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
		if len(branches) == 0 {
			seen[pkg] = true
		}
		for i, branch := range branches {
			if len(branch) > 0 {
				branches[i] = append([]*packages.Package{pkg}, branch...)
			}
		}

		return branches
	}
	allBranches := [][]*packages.Package{}
	for _, pkg := range allPackages {
		branches := visit(pkg)
		if len(branches) == 0 {
			continue
		}
		for _, branch := range branches {
			allBranches = append(allBranches, append([]*packages.Package{{PkgPath: pkg.Module.Path}}, branch...))
		}
	}

	roots := mergeBranches(allBranches)
	for _, root := range roots {
		lw := list.NewWriter()
		lw.SetStyle(list.StyleConnectedLight)
		buildTree(root, lw)
		println(lw.Render())
	}
}

type treeNode struct {
	pkg     *packages.Package
	imports []*treeNode
}

func mergeBranches(branches [][]*packages.Package) []*treeNode {
	roots := make(map[string]*treeNode)

	for _, branch := range branches {
		root := branch[0]
		if _, ok := roots[root.PkgPath]; !ok {
			roots[root.PkgPath] = &treeNode{pkg: root}
		}
		currentNode := roots[root.PkgPath]
		for _, pkg := range branch[1:] {
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

	var rootNodes []*treeNode
	for _, root := range roots {
		rootNodes = append(rootNodes, root)
	}
	sort.Slice(rootNodes, func(i, j int) bool {
		return rootNodes[i].pkg.PkgPath < rootNodes[j].pkg.PkgPath
	})
	return rootNodes
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
