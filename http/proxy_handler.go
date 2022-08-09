package http

import (
	"github.com/gin-gonic/gin"
	"go_gw/infrastructure"
	"net/http"
)

func HandleProxy(ctx *gin.Context) {
	proxy, err := infrastructure.Proxy(ctx)
	if err != nil {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}
	proxy.ServeHTTP(ctx.Writer, ctx.Request)
	return
}
