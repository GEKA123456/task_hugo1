package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getProxyRouter(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, Hugo!")
	})

	// Создаем тестовый сервер
	server := httptest.NewServer(handler)
	defer server.Close()

	router := getProxyRouter(server.URL, "")
	ts := httptest.NewServer(router.r)
	defer ts.Close()

	tests := []struct {
		name   string
		server *httptest.Server
		arg    string
		want   string
		status int
	}{
		{"1", ts, "/api/", "Hello from API", http.StatusOK},
		{"2", ts, "", "Hello, Hugo!\n", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := http.Get(ts.URL + tt.arg)
			assert.NoError(t, err)
			assert.Equal(t, tt.status, res.StatusCode)
			buf := new(bytes.Buffer)
			buf.ReadFrom(res.Body)
			assert.Equal(t, tt.want, buf.String())
		})
	}
}
