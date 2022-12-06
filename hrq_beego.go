package hrq

import (
	"github.com/beego/beego/v2/server/web"
	"github.com/beego/beego/v2/server/web/context"
	"net/http"
)

type beegoAdapter struct {
	ctx    *context.Context
	hrq    *hrq
	reqRsp *reqrsq
}

func (b *beegoAdapter) Abort() interface{}{
	return nil
}

func (b *beegoAdapter) Next() *httpError {
	c := b.ctx
	next := b.reqRsp.req.Context().Value(hrqFilterNextKey)
	if next != nil {
		next.(web.FilterFunc)(c)

	} else {
		c.ResponseWriter.WriteHeader(http.StatusNotFound)
	}
	return nil
}

var _ adapter = (*beegoAdapter)(nil)
