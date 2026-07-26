package framework

import (
	"net/http/pprof"

	"github.com/gin-gonic/gin"
)

// mountPProf registers the standard library pprof handlers under prefix.
// These endpoints expose process internals and must never be reachable
// publicly.
func mountPProf(router *gin.Engine, prefix string) {
	group := router.Group(prefix)
	group.GET("/", gin.WrapF(pprof.Index))
	group.GET("/cmdline", gin.WrapF(pprof.Cmdline))
	group.GET("/profile", gin.WrapF(pprof.Profile))
	group.GET("/symbol", gin.WrapF(pprof.Symbol))
	group.POST("/symbol", gin.WrapF(pprof.Symbol))
	group.GET("/trace", gin.WrapF(pprof.Trace))
	for _, profile := range []string{"allocs", "block", "goroutine", "heap", "mutex", "threadcreate"} {
		group.GET("/"+profile, gin.WrapH(pprof.Handler(profile)))
	}
}
