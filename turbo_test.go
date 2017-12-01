package turbo_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/martai/turbo"
)

func TestTurbo(t *testing.T) {
	render := turbo.New("fixtures/basic", "layout")

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
}
