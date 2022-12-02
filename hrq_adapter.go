package hrq

import (
	"context"
	beecontext "github.com/beego/beego/v2/server/web/context"
	"github.com/gin-gonic/gin"
	"github.com/labstack/echo/v4"
	"net/http"
)

type adapter interface {
	Abort()
	Next() *httpError
}

type defaultAdapter struct {
	ctx    context.Context
	hrq    *hrq
	reqRsp *reqrsq
}

func (d *defaultAdapter) Abort() {
	panic("implement me")
}

func (d *defaultAdapter) Next() *httpError {
	hander, _, _ := d.hrq.router.Lookup(d.reqRsp.method, d.reqRsp.path)
	if hander != nil {
		hander(d.reqRsp.rsp, d.reqRsp.req, nil)
		d.reqRsp.done <- true
	} else {
		d.reqRsp.rsp.WriteHeader(http.StatusNotFound)
		d.reqRsp.rsp.Write([]byte("not found"))
		d.reqRsp.done <- true
	}
	return nil
}

var _ adapter = (*defaultAdapter)(nil)

func (h *hrq) getAdapter(ctx interface{}, reqRsp *reqrsq, hrq *hrq) adapter {
	switch ctx.(type) {
	case *gin.Context:
		return &ginAdapter{ctx.(*gin.Context), hrq, reqRsp}
	case *beecontext.Context:
		return &beegoAdapter{ctx.(*beecontext.Context), hrq, reqRsp}
	case echo.Context:
		return &echoAdapter{ctx.(echo.Context), hrq, reqRsp}
	default:
		return &defaultAdapter{ctx.(context.Context), hrq, reqRsp}
	}
}
