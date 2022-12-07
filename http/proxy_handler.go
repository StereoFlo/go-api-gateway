package http

import (
	"github.com/gin-gonic/gin"
	"go_gw/infrastructure"
	"log"
	"net/http"
)

func HandleProxy(ctx *gin.Context) {
	proxy := infrastructure.NewProxy(ctx)
	err := make(chan error)
	go proxy.ReverseProxy(err)
	res := <-err
	if res != nil {
		responder := infrastructure.NewResponder()
		log.Println(res)
		ctx.AbortWithStatusJSON(http.StatusNotFound, responder.Fail(res.Error()))
		return
	}
	return
}
