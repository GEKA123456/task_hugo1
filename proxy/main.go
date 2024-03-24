package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/go-chi/chi"
)

type Router struct {
	r *chi.Mux
}

func main() {
	host := "http://hugo"
	port := ":1313"
	r := getProxyRouter(host, port)
	//go WorkerTest()
	http.ListenAndServe(":8080", r.r)
}

func getProxyRouter(host, port string) *Router {
	r := &Router{r: chi.NewRouter()}

	r.r.Use(NewReverseProxy(host, port).ReverseProxy)

	r.r.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello from API"))
	})
	return r
}

type ReverseProxy struct {
	host string
	port string
}

func NewReverseProxy(host, port string) *ReverseProxy {
	return &ReverseProxy{
		host: host,
		port: port,
	}
}

// Если ресурс имеет префикс /api/, то запрос должен выдавать текст «Hello from API». Все остальные запросы должны перенаправляться на http://hugo:1313 (сервер hugo).
func (rp *ReverseProxy) ReverseProxy(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/" {
			next.ServeHTTP(w, r)
			return
		}

		targetURL, _ := url.Parse(rp.host + rp.port)

		proxy := httputil.NewSingleHostReverseProxy(targetURL)
		//proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		//	w.WriteHeader(http.StatusBadGateway)
		//	fmt.Fprintln(w, "Ошибка сервера:", err)
		//}
		proxy.ServeHTTP(w, r)
	})
}

/*
const content = ``

func WorkerTest() {
	t := time.NewTicker(1 * time.Second)
	var b byte = 0
	for {
		select {
		case <-t.C:
			err := os.WriteFile("/app/static/_index.md", []byte(fmt.Sprintf(content, b)), 0644)
			if err != nil {
				log.Println(err)
			}
			b++
		}
	}
}
*/
