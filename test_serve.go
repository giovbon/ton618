package main

import (
	"net/http"
	"net/http/httptest"
	"fmt"
)

func main() {
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	w.Header().Set("Content-Type", "text/css")
	http.ServeFile(w, req, "web/static/app.css")

	res := w.Result()
	fmt.Println("Content-Type:", res.Header.Get("Content-Type"))
}
