package hrq

import (
	"bufio"
	"context"
	"github.com/gin-gonic/gin"
	"github.com/julienschmidt/httprouter"
	"io"
	"log"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	// import uuid
)

type reqrsq struct {
	uuid   string
	path   string
	method string
	req    *http.Request
	rsp    http.ResponseWriter
	done   chan bool
}

// define http request queue
var chanReqRsp = make(chan *reqrsq, 1000)

var router = httprouter.New()
var mux = http.NewServeMux()

const (
	noWritten     = -1
	defaultStatus = http.StatusOK
)

type responseWriter struct {
	http.ResponseWriter
	size   int
	status int
}

func (w *responseWriter) reset(writer http.ResponseWriter) {
	w.ResponseWriter = writer
	w.size = noWritten
	w.status = defaultStatus
}

func (w *responseWriter) WriteHeader(code int) {
	if code > 0 && w.status != code {
		if w.Written() {
			//log.Printf("[WARNING] Headers were already written. Wanted to override status code %d with %d", w.status, code)
		}
		w.status = code
	}
}

func (w *responseWriter) WriteHeaderNow() {
	if !w.Written() {
		w.size = 0
		w.ResponseWriter.WriteHeader(w.status)
	}
}

func (w *responseWriter) Write(data []byte) (n int, err error) {
	w.WriteHeaderNow()
	n, err = w.ResponseWriter.Write(data)
	w.size += n
	return
}

func (w *responseWriter) WriteString(s string) (n int, err error) {
	w.WriteHeaderNow()
	n, err = io.WriteString(w.ResponseWriter, s)
	w.size += n
	return
}

func (w *responseWriter) Status() int {
	return w.status
}

func (w *responseWriter) Size() int {
	return w.size
}

func (w *responseWriter) Written() bool {
	return w.size != noWritten
}

// Hijack implements the http.Hijacker interface.
func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if w.size < 0 {
		w.size = 0
	}
	return w.ResponseWriter.(http.Hijacker).Hijack()
}

// CloseNotify implements the http.CloseNotifier interface.
func (w *responseWriter) CloseNotify() <-chan bool {
	return w.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

// Flush implements the http.Flusher interface.
func (w *responseWriter) Flush() {
	w.WriteHeaderNow()
	w.ResponseWriter.(http.Flusher).Flush()
}

func (w *responseWriter) Pusher() (pusher http.Pusher) {
	if pusher, ok := w.ResponseWriter.(http.Pusher); ok {
		return pusher
	}
	return nil
}

func init() {
	// http queue consumer
	for i := 0; i < runtime.NumCPU()*10; i++ {
		go func() {
			handler := func() {
				defer func() {
					if err := recover(); err != nil {
						log.Println(err) //log
					}
				}()
				for {
					select {
					case reqRsp := <-chanReqRsp:
						ctx := reqRsp.req.Context().Value(gin.ContextKey)
						if ctx != nil {
							c := ctx.(*gin.Context)
							c.Request = reqRsp.req
							c.Writer = &responseWriter{ResponseWriter: reqRsp.rsp}
							c.Next()
							reqRsp.done <- true
						} else {
							hander, _, _ := router.Lookup(reqRsp.method, reqRsp.path)
							if hander != nil {
								hander(reqRsp.rsp, reqRsp.req, nil)
								reqRsp.done <- true
							} else {
								reqRsp.rsp.WriteHeader(http.StatusNotFound)
								reqRsp.rsp.Write([]byte("not found"))
								reqRsp.done <- true
							}
						}
					}
				}

			}

			for {
				handler()
			}

		}()
	}
	//mux := http.NewServeMux()
	mux.HandleFunc("/", ServeHTTP)

}

// define default http handler
func ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// define request response struct
	//judge request is multipart/form-data
	if r.Header.Get("Content-Type") == "multipart/form-data" {
		//save request body to file
	}
	c := make(chan bool, 1)
	reqRsp := &reqrsq{
		uuid:   "uuid",
		path:   r.URL.Path,
		method: r.Method,
		req:    r,
		rsp:    w,
		done:   c,
	}
	// push request response struct into queue
	chanReqRsp <- reqRsp
	<-c
}

//// gen uuid
//func genUUID() string {
//
//	time.Now().UnixMilli()
//
//}

var handlerMap = make(map[string]http.HandlerFunc)

var lock sync.RWMutex

func Handle(method string, path string, handler http.HandlerFunc) {

	lock.Lock()
	defer lock.Unlock()
	handlerMap[method+"$"+path] = handler

	h := func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		handler(w, r)
	}
	router.Handle(method, path, h)
}

// GET is a shortcut for router.Handle(http.MethodGet, path, handle)
func GET(path string, handle http.HandlerFunc) {
	Handle(http.MethodGet, path, handle)
}

// HEAD is a shortcut for router.Handle(http.MethodHead, path, handle)
func HEAD(path string, handle http.HandlerFunc) {
	Handle(http.MethodHead, path, handle)
}

// OPTIONS is a shortcut for router.Handle(http.MethodOptions, path, handle)
func OPTIONS(path string, handle http.HandlerFunc) {
	Handle(http.MethodOptions, path, handle)
}

// POST is a shortcut for router.Handle(http.MethodPost, path, handle)
func POST(path string, handle http.HandlerFunc) {
	Handle(http.MethodPost, path, handle)
}

// PUT is a shortcut for router.Handle(http.MethodPut, path, handle)
func PUT(path string, handle http.HandlerFunc) {
	Handle(http.MethodPut, path, handle)
}

// PATCH is a shortcut for router.Handle(http.MethodPatch, path, handle)
func PATCH(path string, handle http.HandlerFunc) {
	Handle(http.MethodPatch, path, handle)
}

// DELETE is a shortcut for router.Handle(http.MethodDelete, path, handle)
func DELETE(path string, handle http.HandlerFunc) {
	Handle(http.MethodDelete, path, handle)
}

func Router() *httprouter.Router {
	return router
}

func ApplyForGin(ginEngine *gin.Engine) {
	lock.RLock()
	defer lock.RUnlock()
	for k, v := range handlerMap {
		method := strings.Split(k, "$")[0]
		path := strings.Split(k, "$")[1]
		ginEngine.Handle(method, path, func(c *gin.Context) {
			v(c.Writer, c.Request)
		})
	}
}

func MiddlewareForGin() gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.Request.WithContext(context.WithValue(c.Request.Context(), gin.ContextKey, c))
		ServeHTTP(c.Writer, req)
	}
}

func ListenAndServe(addr string) error {

	return http.ListenAndServe(addr, mux)
}
