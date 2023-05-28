package app1

import (
	"gitee.com/geektime-geekbang/geektime-go/cache"
	"gitee.com/geektime-geekbang/geektime-go/web"
	"net/http"
	"testing"
	"time"
)

func TestApp1Server(t *testing.T) {
	sess := cache.NewBuildInMapCache(time.Minute * 15)
	server := web.NewHTTPServer(web.ServerWithMiddleware(NewLoginMiddlewareBuilder(sess).Middleware))

	server.Get("/profile", func(ctx *web.Context) {
		ctx.RespString(http.StatusOK, "这是 APP1，你进来啦")
	})

	if err := server.Start(":8081"); err != nil {
		panic(err)
	}
}
