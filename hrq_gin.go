package hrq

import "github.com/gin-gonic/gin"

type ginAdapter struct {
	ctx    *gin.Context
	hrq    *hrq
	reqRsp *reqrsq
}

func (g *ginAdapter) Abort() interface{} {
	return nil
}

func (g *ginAdapter) Next() *httpError {
	c := g.ctx
	c.Request = g.reqRsp.req
	c.Writer = &responseWriter{ResponseWriter: g.reqRsp.rsp}
	g.ctx.Next()
	return nil
}

var _ adapter = (*ginAdapter)(nil)
