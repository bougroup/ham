package ham

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const DefaultOutputDir = "./public"
const configFileName = "ham.json"
const defaultCompileJSON = `{}`
const defaultLayout = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>HAM</title>
  <link type="ham/layout-css"/>
</head>
<body>

<div id="app-info"></div>
<div class="container">
  <div class="row">
      <embed type="ham/page"/>
  </div>
</div>
<embed type="ham/layout-js"/>
</body>
</html>
`
const defaultPage = `<div class="page"
	data-ham-page-config='{
      "layout": "default.lhtml"
     }'
>
  <h1>Welcome to HAM</h1>
</div>`

const defaultTsConfig = `{
  "compilerOptions": {
    "target": "es2020",                                  /* Set the JavaScript language version for emitted JavaScript and include compatible library declarations. */
    "module": "node16",                                /* Specify what module code is generated. */
	"moduleResolution": "Node",
    "esModuleInterop": true,                             /* Emit additional JavaScript to ease support for importing CommonJS modules. This enables 'allowSyntheticDefaultImports' for type compatibility. */
    "forceConsistentCasingInFileNames": true,            /* Ensure that casing is correct in imports. */
    "strict": true,                                      /* Enable all strict type-checking options. */
    "skipLibCheck": true,                                 /* Skip type checking all .d.ts files. */
    "verbatimModuleSyntax": true,
    "outDir": "./public/assets/js"
  },
  "include": ["./src"],
  "exclude": ["./node_modules", "./public"]
}`
const tsconfigFileName = "tsconfig.json"
const defaultPackageJSON = `{
	  "name": "%s",
	  "version": "1.0.0",
	  "description": "A HAM Application",
	  "type": "module",
	  "scripts": {
		"build": "ham build && rollup -c",
		"test": "echo \"Error: no test specified\" && exit 1"
	  },
	  "devDependencies": {
		"@rollup/plugin-node-resolve": "^15.2.3",
		"rollup": "3.17.3",
		"rollup-plugin-copy": "3.4.0",
		"rollup-plugin-typescript2": "^0.36.0",
		"@rollup/plugin-commonjs": "^26.0.1",
		"glob": "^11.0.0"
	  }
}`

const defaultGitIgnore = `node_modules`
const defaultRollupConfig = `import typescript from 'rollup-plugin-typescript2';
import { nodeResolve } from '@rollup/plugin-node-resolve';
import copy from 'rollup-plugin-copy';
import commonjs from '@rollup/plugin-commonjs';
import {glob} from 'glob';

const inputFiles = glob.sync('./src/*.ts'); // Adjust the pattern as needed
export default {
    input: inputFiles,
    output: {
        dir: 'public/assets/js',
        format: 'esm',
        sourcemap: false,
        preserveModules: true,  // Preserve module structure
        preserveModulesRoot: 'src',  // Keep module structure relative to 'src'
    },
    plugins: [
        copy({
            targets: [
                {src: 'src/*.css', dest: 'public/assets/css'},
                {src: 'src/*.js', dest: 'public/assets/js'}
            ]
        }),
        typescript({
            tsconfig: './tsconfig.json'
        }),
        nodeResolve(), // This plugin allows Rollup to resolve modules from node_modules
        commonjs() // Converts CommonJS modules to ES modules
    ]
}`

const srcDir = "src"

var siteStructure = []string{
	"public/assets/img",
	"src",
}

type Site struct {
	host string
	port int
}

func NewSite() *Site {
	return &Site{
		host: "localhost",
		port: 4120,
	}
}

func (h *Site) NewProject(siteName, workingDir string) error {
	// generate folder structure
	for _, folder := range siteStructure {
		folder = strings.ReplaceAll(folder, "{site-name}", siteName)
		if err := os.MkdirAll(filepath.Join(workingDir, siteName, folder), 0744); err != nil {
			return err
		}
	}

	// write default layout
	if err := createFile(filepath.Join(workingDir, siteName, srcDir, "default.lhtml"), []byte(defaultLayout), false); err != nil {
		return err
	}

	// write default index.html
	if err := createFile(filepath.Join(workingDir, siteName, srcDir, "index.html"), []byte(defaultPage), false); err != nil {
		return err
	}

	// write default index.css
	if err := createFile(filepath.Join(workingDir, siteName, srcDir, "index.css"), []byte(""), false); err != nil {
		return err
	}

	// write default index.ts
	if err := createFile(filepath.Join(workingDir, siteName, srcDir, "index.ts"), []byte(""), false); err != nil {
		return err
	}

	// write default tsconfig.json
	if err := createFile(filepath.Join(workingDir, siteName, tsconfigFileName), []byte(defaultTsConfig), false); err != nil {
		return err
	}

	// write default package.json
	if err := createFile(filepath.Join(workingDir, siteName, "package.json"), []byte(fmt.Sprintf(defaultPackageJSON, siteName)), false); err != nil {
		return err
	}

	// write default rollup.config.js
	if err := createFile(filepath.Join(workingDir, siteName, "rollup.config.js"), []byte(defaultRollupConfig), true); err != nil {
		return err
	}

	// write default .gitignore
	if err := createFile(filepath.Join(workingDir, siteName, ".gitignore"), []byte(defaultGitIgnore), true); err != nil {
		return err
	}

	// write default ham.json
	return createFile(filepath.Join(workingDir, siteName, configFileName), []byte(defaultCompileJSON), true)
}

func (h *Site) Build(workingDir, outputDir string) error {
	c, err := New(workingDir, outputDir)
	if err != nil {
		return err
	}
	return c.Compile()
}

func (h *Site) Help() string {
	return `usage: ham <command> [<options>]

The following are supported HAM commands:
  init		Creates a new HAM site
  build		Compiles HAM site into html website
  serve		Starts dev server with hot reload
  version	Displays version of HAM that you are running
`
}
