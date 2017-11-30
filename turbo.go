package turbo

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultLeftDelim  = "{{"
	DefaultRightDelim = "}}"
)

// Included helper functions for use when rendering HTML.
var helperFuncs = template.FuncMap{
	"yield": func() (string, error) {
		return "", fmt.Errorf("yield called with no layout defined")
	},
}

type Render struct {
	Directory     string
	Layout        string
	Extensions    []string
	Funcs         []template.FuncMap
	IsDevelopment bool

	templates *template.Template
}

func New(directory string, layout string) *Render {
	r := &Render{
		Directory: directory,
		Layout:    layout,
	}

	r.prepareRender()
	r.compileTemplatesFromDir()

	return r
}

func (r *Render) HTML(w http.ResponseWriter, status int, name string, binding interface{}) error {
	// If we're in development mode, recompile the templates.
	if r.IsDevelopment {
		r.compileTemplatesFromDir()
	}

	// TODO(ben)
	// If Turbolinks is enabled, render the template without the layout.

	// Assign a layout if there is one.
	if r.Layout != "" {
		r.addLayoutFuncs(name, binding)
		name = r.Layout
	}

	// Execute the template.
	//
	// Note that this fails in interesting ways -- if the template can't be
	// found, we should fail, but all that happens right now is only the part
	// the "layout" template _before_ the {{ yield }} is rendered.
	w.WriteHeader(status)
	r.templates.ExecuteTemplate(w, name, binding)

	return nil
}

func (r *Render) execute(name string, binding interface{}) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	return buf, r.templates.ExecuteTemplate(buf, name, binding)
}

func (r *Render) addLayoutFuncs(name string, binding interface{}) {
	funcs := template.FuncMap{
		"yield": func() (template.HTML, error) {
			buf, err := r.execute(name, binding)
			// Return safe HTML here since we are rendering our own template.
			return template.HTML(buf.String()), err
		},
	}

	if tpl := r.templates.Lookup(name); tpl != nil {
		tpl.Funcs(funcs)
	}
}

func (r *Render) prepareRender() {
	if r.Directory == "" {
		wd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		r.Directory = wd
	}

	if r.Layout == "" {
		r.Layout = "layout"
	}

	if len(r.Extensions) < 1 {
		r.Extensions = []string{".html", ".tmpl"}
	}
}

// compileTemplatesFromDir compiles all of the templates under the given
// directory.
//
// This is (mostly) a copy of
// https://github.com/unrolled/render/blob/v1/render.go#L185, since they do it
// the best.
func (r *Render) compileTemplatesFromDir() {
	r.templates = template.New(r.Directory)
	r.templates.Delims(DefaultLeftDelim, DefaultRightDelim)

	// Walk the directory and compile any valid template.
	filepath.Walk(r.Directory, func(path string, info os.FileInfo, err error) error {
		// If we encounter a directory, return immediately since we can't
		// compile it.
		if info == nil || info.IsDir() {
			return nil
		}

		// Get the path relative to our root template directory.
		rel, err := filepath.Rel(r.Directory, path)
		if err != nil {
			return err
		}

		// Determine the file extension.
		ext := ""
		if strings.Index(rel, ".") != -1 {
			ext = filepath.Ext(rel)
		}

		// Compile each template. We check if the extension matches the
		// allowed ones that we defined before compiling.
		for _, extension := range r.Extensions {
			if ext == extension {
				buf, err := ioutil.ReadFile(path)
				if err != nil {
					panic(err)
				}

				name := (rel[0 : len(rel)-len(ext)])
				tmpl := r.templates.New(filepath.ToSlash(name))

				// Add our funcmaps.
				for _, funcs := range r.Funcs {
					tmpl.Funcs(funcs)
				}

				// Break out if this parsing fails. We don't want any silent
				// server starts.
				template.Must(tmpl.Funcs(helperFuncs).Parse(string(buf)))
				break
			}
		}

		return nil
	})
}
