package http

import (
	"github.com/gin-gonic/gin"
	infrastructure2 "go_gw/pkg/infrastructure"
	"net/http"
)

func HandleProxy(ctx *gin.Context) {
	responder := infrastructure2.NewResponder()
	proxy := infrastructure2.NewProxy(ctx)
	errCh := make(chan error)
	go proxy.ReverseProxy(errCh)
	res := <-errCh
	if res != nil {
		ctx.AbortWithStatusJSON(http.StatusNotFound, responder.Fail(res.Error()))
		return
	}
	return
}
