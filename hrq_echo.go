package hrq

import (
	"github.com/labstack/echo/v4"
	"strings"
)

type echoAdapter struct {
	ctx    echo.Context
	hrq    *hrq
	reqRsp *reqrsq
}

func (e *echoAdapter) Abort() {
	//panic("implement me")
}

func (e *echoAdapter) Next() *httpError {
	c := e.ctx
	next := e.reqRsp.req.Context().Value(hrqFilterNext)
	if next != nil {
		if err := next.(echo.HandlerFunc)(c); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "not found") {
				return errNotFound
			}
		}
	}
	return nil
}

var _ adapter = (*echoAdapter)(nil)
