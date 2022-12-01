package hrq

import "github.com/labstack/echo/v4"

type echoAdapter struct {
	ctx    echo.Context
	hrq    *hrq
	reqRsp *reqrsq
}

func (e *echoAdapter) Abort() {
	//panic("implement me")
}

func (e *echoAdapter) Next() error {
	c := e.ctx
	next := e.reqRsp.req.Context().Value(hrqFilterNext)
	if next != nil {
		return next.(echo.HandlerFunc)(c)
	}
	return nil
}

var _ adapter = (*echoAdapter)(nil)
