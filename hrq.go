package hrq

import (
	"github.com/beego/beego/v2/server/web"
	"github.com/gin-gonic/gin"
	"github.com/julienschmidt/httprouter"
	"github.com/labstack/echo/v4"
	"net/http"
	"runtime"
)

var Conf = &Config{
	Workers:       runtime.NumCPU() * 10,
	MaxQueueSize:  runtime.NumCPU() * 100,
	MaxConnection: 10000,
	//TimeoutProcess: 0,
	TimeoutQueue:     3000,
	TempDir:          "/tmp",
	EnableOverload:   true,
	PerRequestQueue:  runtime.NumCPU() * 10,
	PerRequestWorker: runtime.NumCPU() * 5,
	workerOptions: make(map[string]workerOption),

}

var ghrp *hrq = New(Conf)

// global

func GET(path string, handle http.HandlerFunc,options ...workerOption) {
	ghrp.Handle(http.MethodGet, path, handle,options...)
}

// HEAD is a shortcut for router.Handle(http.MethodHead, path, handle)
func HEAD(path string, handle http.HandlerFunc,options ...workerOption) {
	ghrp.Handle(http.MethodHead, path, handle,options...)
}

// OPTIONS is a shortcut for router.Handle(http.MethodOptions, path, handle)
func OPTIONS(path string, handle http.HandlerFunc,options ...workerOption) {
	ghrp.Handle(http.MethodOptions, path, handle,options...)
}

// POST is a shortcut for router.Handle(http.MethodPost, path, handle)
func POST(path string, handle http.HandlerFunc,options ...workerOption) {
	ghrp.Handle(http.MethodPost, path, handle,options...)
}

// PUT is a shortcut for router.Handle(http.MethodPut, path, handle)
func PUT(path string, handle http.HandlerFunc,options ...workerOption) {
	ghrp.Handle(http.MethodPut, path, handle,options...)
}

// PATCH is a shortcut for router.Handle(http.MethodPatch, path, handle)
func PATCH(path string, handle http.HandlerFunc,options ...workerOption) {
	ghrp.Handle(http.MethodPatch, path, handle,options...)
}

// DELETE is a shortcut for router.Handle(http.MethodDelete, path, handle)
func DELETE(path string, handle http.HandlerFunc,options ...workerOption) {
	ghrp.Handle(http.MethodDelete, path, handle,options...)
}

func Router() *httprouter.Router {
	return ghrp.router
}

func ApplyToGin(ginEngine *gin.Engine) {
	ghrp.ApplyToGin(ginEngine)
}

func ApplyToBeego(server *web.HttpServer) {
	ghrp.ApplyToBeego(server)
}

func ApplyFromGin(ginEngine *gin.Engine) {
	ghrp.ApplyFromGin(ginEngine)

}

func MiddlewareForGin() gin.HandlerFunc {
	return ghrp.MiddlewareForGin()
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

	ghrp.InstallFilterChanForBeego()
}

func MiddlewareForEcho() echo.MiddlewareFunc {
	return ghrp.MiddlewareForEcho()
}

func ApplyToEcho(e *echo.Echo) {

	ghrp.ApplyToEcho(e)
}

func ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, ghrp.mux)
}
