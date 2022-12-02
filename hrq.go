package hrq

import (
	"context"
	"github.com/beego/beego/v2/server/web"
	beecontext "github.com/beego/beego/v2/server/web/context"
	"github.com/gin-gonic/gin"
	"github.com/julienschmidt/httprouter"
	"github.com/labstack/echo/v4"
	"net/http"
	"runtime"
	"strings"
)



var Conf = &Config{
	Workers:        runtime.NumCPU() * 10,
	MaxQueueSize:   runtime.NumCPU() * 100,
	MaxConnection:  1000,
	TimeoutProcess: 0,
	TimeoutQueue:   0,
	TempDir:        "/tmp",
}

var ghrp *hrq = New(Conf)

// global

func GET(path string, handle http.HandlerFunc) {
	ghrp.Handle(http.MethodGet, path, handle)
}

// HEAD is a shortcut for router.Handle(http.MethodHead, path, handle)
func HEAD(path string, handle http.HandlerFunc) {
	ghrp.Handle(http.MethodHead, path, handle)
}

// OPTIONS is a shortcut for router.Handle(http.MethodOptions, path, handle)
func OPTIONS(path string, handle http.HandlerFunc) {
	ghrp.Handle(http.MethodOptions, path, handle)
}

// POST is a shortcut for router.Handle(http.MethodPost, path, handle)
func POST(path string, handle http.HandlerFunc) {
	ghrp.Handle(http.MethodPost, path, handle)
}

// PUT is a shortcut for router.Handle(http.MethodPut, path, handle)
func PUT(path string, handle http.HandlerFunc) {
	ghrp.Handle(http.MethodPut, path, handle)
}

// PATCH is a shortcut for router.Handle(http.MethodPatch, path, handle)
func PATCH(path string, handle http.HandlerFunc) {
	ghrp.Handle(http.MethodPatch, path, handle)
}

// DELETE is a shortcut for router.Handle(http.MethodDelete, path, handle)
func DELETE(path string, handle http.HandlerFunc) {
	ghrp.Handle(http.MethodDelete, path, handle)
}

func Router() *httprouter.Router {
	return ghrp.router
}

func ApplyToGin(ginEngine *gin.Engine) {
	ghrp.handlerLock.RLock()
	defer ghrp.handlerLock.RUnlock()
	for k, _ := range ghrp.handlerMap {
		method := strings.Split(k, "$")[0]
		path := strings.Split(k, "$")[1]
		ginEngine.Handle(method, path, func(c *gin.Context) {
			if handler, _, _ := ghrp.router.Lookup(method, path); handler != nil {
				handler(c.Writer, c.Request, nil)
			} else {
				c.Writer.WriteHeader(http.StatusNotFound)
			}
		})
	}
}

func ApplyToBeego(server *web.HttpServer) {
	ghrp.handlerLock.RLock()
	defer ghrp.handlerLock.RUnlock()
	for k, _ := range ghrp.handlerMap {
		method := strings.Split(k, "$")[0]
		path := strings.Split(k, "$")[1]
		server.Handlers.AddMethod(method, path, func(ctx *beecontext.Context) {
			if handler, _, _ := ghrp.router.Lookup(method, path); handler != nil {
				handler(ctx.ResponseWriter, ctx.Request, nil)
			} else {
				ctx.ResponseWriter.WriteHeader(http.StatusNotFound)
			}
		})
	}
}

func ApplyFromGin(ginEngine *gin.Engine) {
	for _, v := range ginEngine.Routes() {
		ghrp.Handle(v.Method, v.Path, func(w http.ResponseWriter, r *http.Request) {
			ctx := &gin.Context{}
			r = r.WithContext(context.WithValue(r.Context(), hrqContextKey, ctx))
			ctx.Request = r
			ctx.Writer = &responseWriter{ResponseWriter: w}
			v.HandlerFunc(ctx)
		})
	}
}

func MiddlewareForGin() gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.Request.WithContext(context.WithValue(c.Request.Context(), hrqContextKey, c))
		ghrp.ServeHTTP(c.Writer, req)
	}
}

func MiddlewareForEcho() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			c := context.WithValue(ctx.Request().Context(), hrqContextKey, ctx)
			c = context.WithValue(c, hrqFilterNext, next)
			ctx.SetRequest(ctx.Request().WithContext(c))
			ghrp.ServeHTTP(ctx.Response().Writer, ctx.Request())
			return nil
		}
	}
}

func ApplyToEcho(e *echo.Echo) {
	ghrp.handlerLock.RLock()
	defer ghrp.handlerLock.RUnlock()
	for k, _ := range ghrp.handlerMap {
		method := strings.Split(k, "$")[0]
		path := strings.Split(k, "$")[1]
		e.Add(method, path, func(c echo.Context) error {
			if handler, _, _ := ghrp.router.Lookup(method, path); handler != nil {
				handler(c.Response().Writer, c.Request(), nil)
			} else {
				c.Response().WriteHeader(http.StatusNotFound)
			}
			return nil
		})
	}
}

// get stat
func GetStat() map[string]*stat {
	return ghrp.GetStat()
}

// set global hrq
func SetGlobalHrq(h *hrq) {
	ghrp = h
}

func InstallFilterChanForBeego() {
	web.InsertFilterChain("/*", func(next web.FilterFunc) web.FilterFunc {
		return func(ctx *beecontext.Context) {
			c := context.WithValue(ctx.Request.Context(), hrqContextKey, ctx)
			c = context.WithValue(c, hrqFilterNext, next)
			ctx.Request = ctx.Request.WithContext(c)
			ghrp.ServeHTTP(ctx.ResponseWriter, ctx.Request)
			next(ctx)
		}
	})
}

func ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, ghrp.mux)
}
