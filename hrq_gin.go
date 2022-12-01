package hrq

import "github.com/gin-gonic/gin"

type ginAdapter struct {
	ctx    *gin.Context
	hrq    *hrq
	reqRsp *reqrsq
}

func (g *ginAdapter) Abort() {
	//panic("implement me")
}

func (g *ginAdapter) Next() error {
	c := g.ctx
	c.Request = g.reqRsp.req
	c.Writer = &responseWriter{ResponseWriter: g.reqRsp.rsp}
	g.ctx.Next()
	return nil
}

var _ adapter = (*ginAdapter)(nil)
