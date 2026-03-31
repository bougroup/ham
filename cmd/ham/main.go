package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fobilow/detach"
	"github.com/fobilow/ham"
	"github.com/fobilow/ham/proxy"
	"github.com/fobilow/ham/serve"
)

var Version string

var validSiteName = regexp.MustCompile(`\W+`)

func main() {
	// attach -d flag to default CommandLine flag set
	cleanup := detach.Setup("d", nil)
	defer cleanup()

	h := ham.NewSite()
	newCmd := newFlagSet(h, "init")
	buildCmd := newFlagSet(h, "build")
	serveCmd := newFlagSet(h, "serve")

	bwd := buildCmd.String("w", "./", "working directory")
	bod := buildCmd.String("o", ham.DefaultOutputDir, "output directory")

	swd := serveCmd.String("w", "./", "working directory")
	sport := serveCmd.String("p", "4120", "port")

	command := ""
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "init":
		name := os.Args[2]
		if len(name) == 0 {
			fmt.Println("please provide a name for your site")
			newCmd.Usage()
			return
		}
		if validSiteName.MatchString(name) {
			fmt.Println(name)
			fmt.Println("invalid project name. project name can only contains letters, digits or underscore")
			newCmd.Usage()
			return
		}
		checkError(h.NewProject(name, getWorkingDir("./")))
	case "build":
		checkError(buildCmd.Parse(os.Args[2:]))
		if len(*bwd) == 0 {
			fmt.Println("please provide a working directory")
			buildCmd.Usage()
			return
		}
		checkError(h.Build(getWorkingDir(*bwd), *bod))
	case "serve":
		checkError(serveCmd.Parse(os.Args[2:]))
		serve.Run(getWorkingDir(*swd), *sport)
	case "proxy":
		proxy.Run()
	case "version":
		fmt.Println("Version: " + Version)
	default:
		fmt.Println(h.Help())
	}
}

func checkError(err error) {
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func getWorkingDir(wd string) string {
	defaultWorkingDir, err := os.Getwd()
	checkError(err)

	workingDir := wd
	if !strings.HasPrefix(wd, "/") {
		workingDir = filepath.Join(defaultWorkingDir, wd)
	}
	return workingDir
}

func newFlagSet(s *ham.Site, name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.Usage = func() { fmt.Println(s.Help()) }
	return fs
}
