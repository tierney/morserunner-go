package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRouterConfiguration(t *testing.T) {
	// Set Gin to test mode to avoid noise
	gin.SetMode(gin.TestMode)

	// Create a temporary web/dist structure to satisfy StaticFile checks
	os.MkdirAll("web/dist/assets", 0755)
	os.WriteFile("web/dist/index.html", []byte("test"), 0644)
	os.WriteFile("web/dist/vite.svg", []byte("test"), 0644)
	os.WriteFile("web/dist/audio-processor.js", []byte("test"), 0644)
	defer os.RemoveAll("web")

	// This should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Router initialization panicked: %v", r)
		}
	}()

	r := gin.New()

	// Replicate the routing logic from s.Start to test for conflicts
	r.GET("/ws", func(c *gin.Context) {})
	r.Static("/assets", "web/dist/assets")
	r.StaticFile("/", "web/dist/index.html")
	r.StaticFile("/vite.svg", "web/dist/vite.svg")
	r.StaticFile("/audio-processor.js", "web/dist/audio-processor.js")

	// Verify the routes are reachable (at least defined)
	ts := httptest.NewServer(r)
	defer ts.Close()

	res, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", res.StatusCode)
	}
}
