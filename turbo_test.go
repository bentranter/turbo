package turbo_test

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"
	textTpl "text/template"

	"github.com/bentranter/turbo"
)

func TestTurbo(t *testing.T) {
	render := turbo.New(turbo.Options{
		Directory: "fixtures/basic",
		Layout:    "layout",
	})

	t.Run("rendering non-existent template should error", func(t *testing.T) {
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := render.HTML(w, r, http.StatusOK, "not/found", "test"); err == nil {
				t.Fatalf("expected error when rendering non-existent template but got none")
			}
		})

		res := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		h.ServeHTTP(res, req)

		if res.Code != http.StatusInternalServerError {
			t.Fatalf("expected HTTP status %d but got %d", http.StatusInternalServerError, res.Code)
		}
	})

	t.Run("render template without errors", func(t *testing.T) {
		const expected = `head<p>test</p>foot`
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := render.HTML(w, r, http.StatusOK, "content", "test"); err != nil {
				t.Fatalf("unexpected error rendering template: %v", err)
			}
		})

		res := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		h.ServeHTTP(res, req)

		if res.Code != http.StatusOK {
			t.Fatalf("expected HTTP status %d but got %d", http.StatusOK, res.Code)
		}
		body := res.Body.String()
		if body != expected {
			t.Fatalf("expected %s but got %s", expected, body)
		}
	})

	t.Run("render a partial", func(t *testing.T) {
		const expected = `<p>test</p>`
		var err error
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err = render.HTML(w, r, http.StatusOK, "content", "test", true)
		})
		if err != nil {
			t.Fatalf("unexpected error rendering template: %v", err)
		}

		res := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		h.ServeHTTP(res, req)

		if res.Code != http.StatusOK {
			t.Fatalf("expected HTTP status %d but got %d", http.StatusOK, res.Code)
		}
		body := res.Body.String()
		if body != expected {
			t.Fatalf("expected %s but got %s", expected, body)
		}
	})

	t.Run("turbolinks redirect", func(t *testing.T) {
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/redirect", http.StatusFound)
		})

		res := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)

		// Make sure we hit the Turbolinks handler.
		req.Header.Set("Turbolinks-Referrer", "http://localhost:3000/redirect")
		turboh := turbo.Handler(h)
		turboh.ServeHTTP(res, req)

		if res.Code != http.StatusFound {
			t.Fatalf("expected HTTP status %d but got %d", http.StatusFound, res.Code)
		}

		cookieReq := &http.Request{Header: http.Header{"Cookie": res.HeaderMap["Set-Cookie"]}}
		_, err := cookieReq.Cookie(turbo.TurbolinksCookie)
		if err != nil {
			t.Fatalf("expected cookie but got %v", err.Error())
		}
	})

	t.Run("turbolinks form submission", func(t *testing.T) {
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/", http.StatusFound)
		})

		res := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/", nil)

		// Make sure we hit the Turbolinks handler.
		req.Header.Set("Turbolinks-Referrer", "http://localhost:3000/redirect")
		turboh := turbo.Handler(h)
		turboh.ServeHTTP(res, req)

		if res.Code != http.StatusOK {
			t.Fatalf("expected HTTP status %d but got %d", http.StatusOK, res.Code)
		}
		contentType := res.Header().Get("Content-Type")
		if contentType != "text/javascript" {
			t.Fatalf("expected Content-Type to be text/javascript but got %s", contentType)
		}
		expectedJS := `Turbolinks.clearCache();Turbolinks.visit("/", {action: "advance"});`
		actualJS := res.Body.String()
		if actualJS != expectedJS {
			t.Fatalf("expected response to be %s but got %s", expectedJS, actualJS)
		}
	})
}

func TestRender_String(t *testing.T) {
	render := turbo.New(turbo.Options{
		Directory: "fixtures/basic",
		Layout:    "layout",
	})

	t.Run("rendering an non-existent template to a string should error", func(t *testing.T) {
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, err := render.String(w, r, "notFound", nil); err == nil {
				t.Fatalf("expected error rendering non-existent template, but got none")
			}
		})

		res := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		h.ServeHTTP(res, req)
	})

	t.Run("render template to string", func(t *testing.T) {
		const expected = `head<p>test</p>foot`

		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actual, err := render.String(w, r, "content", "test")
			if err != nil {
				t.Fatalf("unexpected error rendering template: %v", err)
			}

			if actual != expected {
				t.Fatalf("expected %s but got %s", expected, actual)
			}
		})

		res := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		h.ServeHTTP(res, req)
	})

	t.Run("render a partial", func(t *testing.T) {
		const expected = `<p>test</p>`
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actual, err := render.String(w, r, "content", "test", true)
			if err != nil {
				t.Fatalf("unexpected error rendering template: %v", err)
			}

			if actual != expected {
				t.Fatalf("expected %s but got %s", expected, actual)
			}
		})

		res := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		h.ServeHTTP(res, req)
	})
}

func TestTurboErrors(t *testing.T) {
	render := turbo.New(turbo.Options{
		Directory: "fixtures/error",
	})

	t.Run("render template with invalid HTML", func(t *testing.T) {
		var err error
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err = render.HTML(w, r, http.StatusOK, "badHTML", nil)
		})

		res := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		h.ServeHTTP(res, req)

		if err == nil {
			t.Fatalf("expected error rendering template but got nothing")
		}
		tplErr, ok := err.(*template.Error)
		if !ok {
			t.Fatalf("expected error to be of type template error, but got %#v", err)
		}
		if tplErr.ErrorCode != 4 {
			t.Fatalf("expetced error code %d but got %d", 4, tplErr.ErrorCode)
		}
	})

	t.Run("render template with invalid data", func(t *testing.T) {
		data := &struct {
			V interface{}
		}{
			V: "test",
		}
		tplName := "badData"

		var err error
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err = render.HTML(w, r, http.StatusOK, tplName, data)
		})

		res := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		h.ServeHTTP(res, req)

		if err == nil {
			t.Fatalf("expected error rendering template but got nothing")
		}
		tplErr, ok := err.(textTpl.ExecError)
		if !ok {
			t.Fatalf("expected error to be of type text/template exec error, but got %#v", err)
		}
		if tplErr.Name != tplName {
			t.Fatalf("expected erroneous template to have name %s but got %s", tplName, tplErr.Name)
		}
	})
}

func TestRender_Flash(t *testing.T) {
	t.Parallel()

	render := turbo.New(turbo.Options{
		Directory: "fixtures/basic",
		Layout:    "layout",
	})

	const message = "test flash message"

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	res := httptest.NewRecorder()

	var raw string
	t.Run("set a flash message", func(t *testing.T) {
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			render.Flash(w, message)
		})
		h.ServeHTTP(res, req)

		raw = res.Header().Get("Set-Cookie")
		if raw == "" {
			t.Fatalf("failed to set cookie on request")
		}
	})

	// Setup.
	header := http.Header{}
	header.Add("Cookie", raw)
	req.Header = header

	t.Run("get a flash message", func(t *testing.T) {
		var flash string
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			flash = render.GetFlash(w, r)
		})
		h.ServeHTTP(res, req)

		if flash != message {
			t.Fatalf("expected flash message to be %s but got %s", message, flash)
		}
	})
}
