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
	// TurbolinksReferrer is the header sent by the Turbolinks frontend on any
	// XHR requests powered by Turbolinks. We use this header to detect if the
	// current request was sent from Turbolinks.
	TurbolinksReferrer = "Turbolinks-Referrer"

	// TurbolinksCookie is the name of the cookie that we use to handle
	// redirect requests correctly.
	//
	// We name it `_turbolinks_location` to be consistent with the name Rails
	// give to the cookie that serves the same purpose.
	TurbolinksCookie = "_turbolinks_location"
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

// HTML renders an HTML template.
func (r *Render) HTML(w http.ResponseWriter, status int, name string, binding interface{}) error {
	// If we're in development mode, recompile the templates.
	if r.IsDevelopment {
		r.compileTemplatesFromDir()
	}

	// Assign a layout if there is one.
	if r.Layout != "" {
		r.addLayoutFuncs(name, binding)
		name = r.Layout
	}

	// Execute the template.
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

// Handler is a middleware wrapper for Turbolinks.
func Handler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		referer := r.Header.Get(TurbolinksReferrer)
		if referer == "" {
			// Turbolinks isn't enabled, so don't do anything extra.
			h.ServeHTTP(w, r)
			return
		}

		if cookie, err := r.Cookie(TurbolinksCookie); err == nil {
			w.Header().Set("Turbolinks-Location", "/")
			cookie.MaxAge = -1
			http.SetCookie(w, cookie)
		}

		// Handle the request. We use a "response staller" here so that,
		//
		//	* The request isn't sent when the underlying http.ResponseWriter
		//	  calls write.
		//	* We can still write to the header after the request is handled.
		//
		// This is done in order to append the `_turbolinks_location` cookie
		// for the requests that need it.
		rs := &responseStaller{
			w:    w,
			code: 0,
			buf:  &bytes.Buffer{},
		}
		h.ServeHTTP(rs, r)

		// Check if a redirect was performed. Is there was, then we need a way
		// to tell the next request to set the special Turbolinks header that
		// will force Turbolinks to update the URL (as push state history) for
		// that redirect. We do this by setting a cookie on this request that
		// we can check on the next request.
		//
		// TODO(ben) Also handle POST redirects properly.
		if location := rs.Header().Get("Location"); location != "" {
			http.SetCookie(rs, &http.Cookie{
				Name:     TurbolinksCookie,
				Value:    "true",
				Path:     "/",
				HttpOnly: true,
			})
		}

		rs.SendResponse()
	})
}

type responseStaller struct {
	w    http.ResponseWriter
	code int
	buf  *bytes.Buffer
}

// Write is a wrapper that calls the underlying response writer's Write
// method, but write the response to a buffer instead.
func (rw *responseStaller) Write(b []byte) (int, error) {
	return rw.buf.Write(b)
}

// WriteHeader saves the status code, to be sent later during the SendReponse
// call.
func (rw *responseStaller) WriteHeader(code int) {
	rw.code = code
}

// Header wraps the underlying response writers Header method.
func (rw *responseStaller) Header() http.Header {
	return rw.w.Header()
}

// SendResponse writes the header to the underlying response writer, and
// writes the response.
func (rw *responseStaller) SendResponse() {
	rw.w.WriteHeader(rw.code)
	rw.buf.WriteTo(rw.w)
}
