package http

import (
	"github.com/gin-gonic/gin"
	"go_gw/infrastructure"
	"net/http"
)

func HandleProxy(ctx *gin.Context) {
	responder := infrastructure.NewResponder()
	proxy := infrastructure.NewProxy(ctx)
	errCh := make(chan error)
	go proxy.ReverseProxy(errCh)
	res := <-errCh
	if res != nil {
		ctx.AbortWithStatusJSON(http.StatusNotFound, responder.Fail(res.Error()))
		return
	}
	return
}
