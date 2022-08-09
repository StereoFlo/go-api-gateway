package http

import (
	"github.com/gin-gonic/gin"
	"go_gw/infrastructure"
	"net/http"
)

func HandleProxy(ctx *gin.Context) {
	proxy := infrastructure.BewProxy(ctx)
	rp, err := proxy.ReverseProxy()
	if err != nil {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}
	rp.ServeHTTP(ctx.Writer, ctx.Request)
	return
}
