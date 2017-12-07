package turbo_test

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"
	textTpl "text/template"

	"github.com/martai/turbo"
)

func TestTurbo(t *testing.T) {
	render := turbo.New(turbo.Options{
		Directory: "fixtures/basic",
		Layout:    "layout",
	})

	t.Run("render template without errors", func(t *testing.T) {
		const expected = `head<p>test</p>foot`
		var err error
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err = render.HTML(w, http.StatusOK, "content", "test")
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

	t.Run("render unnamed template", func(t *testing.T) {
		var err error
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err = render.HTML(w, http.StatusOK, "notFound", "test")
		})
		if err != nil {
			t.Fatalf("expecting error when rendering non-existent template but got nil")
		}

		res := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		h.ServeHTTP(res, req)

		if res.Code == http.StatusInternalServerError {
			t.Fatalf("expected HTTP status %d but got %d", http.StatusInternalServerError, res.Code)
		}
	})

	// TODO(ben) make sure all tests go through Turbolinks middleware.
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
}

func TestTurboErrors(t *testing.T) {
	render := turbo.New(turbo.Options{
		Directory: "fixtures/error",
	})

	t.Run("render template with invalid HTML", func(t *testing.T) {
		var err error
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err = render.HTML(w, http.StatusOK, "badHTML", nil)
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
			err = render.HTML(w, http.StatusOK, tplName, data)
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
