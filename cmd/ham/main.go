package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/fobilow/detach"
	"github.com/fobilow/ham"
	"github.com/fobilow/ham/proxy"
	"github.com/fobilow/ham/serve"
)

var Version string

var validSiteName = regexp.MustCompile(`\W+`)

func main() {
	cleanup := detach.Setup("d", nil)
	defer cleanup()

	site := ham.NewSite()

	buildFlags := newFlagSet(site, "build")
	buildWorkDir := buildFlags.String("w", "./", "working directory")
	buildOutputDir := buildFlags.String("o", ham.DefaultOutputDir, "output directory")

	serveFlags := newFlagSet(site, "serve")
	serveWorkDir := serveFlags.String("w", "./", "working directory")
	servePort := serveFlags.String("p", "4120", "port")

	command := ""
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "init":
		if len(os.Args) < 3 || len(os.Args[2]) == 0 {
			exitWithUsage(site, "please provide a name for your site")
		}
		name := os.Args[2]
		if validSiteName.MatchString(name) {
			exitWithUsage(site, "invalid project name: project name can only contain letters, digits or underscore")
		}
		exitOnError(site.NewProject(name, resolveDir("./")))

	case "build":
		exitOnError(buildFlags.Parse(os.Args[2:]))
		if len(*buildWorkDir) == 0 {
			exitWithUsage(site, "please provide a working directory")
		}
		exitOnError(site.Build(resolveDir(*buildWorkDir), *buildOutputDir))

	case "serve":
		exitOnError(serveFlags.Parse(os.Args[2:]))
		serve.Run(resolveDir(*serveWorkDir), *servePort)

	case "proxy":
		proxy.Run()

	case "version":
		fmt.Println("Version: " + Version)

	default:
		fmt.Println(site.Help())
	}
}

func exitOnError(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func exitWithUsage(site *ham.Site, msg string) {
	fmt.Fprintln(os.Stderr, msg)
	fmt.Println(site.Help())
	os.Exit(1)
}

func resolveDir(dir string) string {
	if filepath.IsAbs(dir) {
		return dir
	}
	cwd, err := os.Getwd()
	exitOnError(err)
	return filepath.Join(cwd, dir)
}

func newFlagSet(site *ham.Site, name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.Usage = func() { fmt.Println(site.Help()) }
	return fs
}
