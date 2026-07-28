package browsertest

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Alfian57/gin-kit/runtime"
	"github.com/Alfian57/gin-kit/runtime/httpx"
	"github.com/gin-gonic/gin"
)

func TestStartServerServesTheApplication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	application, err := runtime.New(runtime.Options{})
	if err != nil {
		t.Fatal(err)
	}
	application.Router().GET("/hello", func(c *gin.Context) {
		httpx.OK(c, gin.H{"message": "from-real-listener"})
	})
	base := StartServer(t, application)

	response, err := http.Get(base + "/hello")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "from-real-listener") {
		t.Fatalf("unexpected response: %d %s", response.StatusCode, body)
	}
}

func TestLaunchSkipsWhenDisabled(t *testing.T) {
	t.Setenv("PLAYWRIGHT_SKIP", "1")
	result := testing.RunTests(func(pattern, value string) (bool, error) { return true, nil },
		[]testing.InternalTest{{
			Name: "skipped",
			F: func(inner *testing.T) {
				Launch(inner)
				inner.Fatal("Launch should have skipped")
			},
		}})
	if !result {
		t.Fatal("inner test failed instead of skipping")
	}
}
