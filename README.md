# Turbo

Turbo is a Go backend for Basecamp's [Turbolinks](https://github.com/turbolinks/turbolinks).

### Usage

Wrap an `http.Handler` in `turbo.Handler`, ie:

```go
package main

import (
    "net/http"

    "github.com/bentranter/turbo"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("hey"))
    })
    http.ListenAndServe(":3000", turbo.Handler(mux))
}
```
