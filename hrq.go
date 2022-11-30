package hrq

import (
	"bufio"
	"context"
	"github.com/beego/beego/v2/server/web"
	beecontext "github.com/beego/beego/v2/server/web/context"
	"github.com/gin-gonic/gin"
	"github.com/julienschmidt/httprouter"
	"github.com/labstack/echo/v4"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
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
//var chanReqRsp = make(chan *reqrsq, 1000)
//
//var router = httprouter.New()
//var mux = http.NewServeMux()
//var tmpDir = "/tmp"

type Config struct {
	// The number of goroutines that will be used to handle requests.
	// If <= 0, then the number of CPUs will be used.
	Workers int
	// The size of the queue that will be used to store requests.
	// If <= 0, then the default value will be used.
	MaxQueueSize int
	// TempDir is the directory to use for temporary files.
	TempDir string
}

var Conf = &Config{
	Workers:      runtime.NumCPU() * 10,
	MaxQueueSize: runtime.NumCPU() * 100,
	TempDir:      "/tmp",
}

type hrq struct {
	once       sync.Once
	mux        *http.ServeMux
	chanReqRsp chan *reqrsq
	router     *httprouter.Router
	config     *Config
}

var ghrp *hrq = New(Conf)

const (
	noWritten     = -1
	defaultStatus = http.StatusOK
	hrqContextKey = "hrqContextKey"
	hrqFilterNext = "hrqFilterNext"
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

func New(conf *Config) *hrq {
	if conf == nil {
		conf = Conf
	}
	h := &hrq{
		mux:        http.NewServeMux(),
		chanReqRsp: make(chan *reqrsq, 1000),
		config:     conf,
		once:       sync.Once{},
		router:     httprouter.New(),
	}
	h.mux.HandleFunc("/", h.ServeHTTP)
	return h
}

func (h *hrq) Init(worker int, queueSize int) {
	// http queue consumer
	h.chanReqRsp = make(chan *reqrsq, h.config.MaxQueueSize)
	for i := 0; i < h.config.Workers; i++ {
		go func() {
			handler := func() {
				defer func() {
					if err := recover(); err != nil {
						log.Println(err) //log
					}
				}()
				for {
					select {
					case reqRsp := <-h.chanReqRsp:
						hrqCxt := reqRsp.req.Context().Value(hrqContextKey)
						switch hrqCxt.(type) {
						case *beecontext.Context:
							c := hrqCxt.(*beecontext.Context)
							next := reqRsp.req.Context().Value(hrqFilterNext)
							if next != nil {
								next.(web.FilterFunc)(c)
							}
							reqRsp.done <- true

						case *gin.Context:
							c := hrqCxt.(*gin.Context)
							c.Request = reqRsp.req
							c.Writer = &responseWriter{ResponseWriter: reqRsp.rsp}
							c.Next()
							reqRsp.done <- true

						case echo.Context:
							c := hrqCxt.(echo.Context)
							next := reqRsp.req.Context().Value(hrqFilterNext)
							if next != nil {
								next.(echo.HandlerFunc)(c)
							}
							reqRsp.done <- true

							reqRsp.done <- true
						default:
							hander, _, _ := h.router.Lookup(reqRsp.method, reqRsp.path)
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
}

// define default http handler
func (h *hrq) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	once.Do(func() {
		h.Init(h.config.Workers, h.config.MaxQueueSize)
	})
	// define request response struct
	//judge request is multipart/form-data

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") && r.Method == "POST" && r.ContentLength > 1024*1024*4 {
		tmpFile, err := ioutil.TempFile(h.config.TempDir, "hrq_upload_")
		if err == nil {
			defer tmpFile.Close()
			defer os.Remove(tmpFile.Name())
			_, err = io.Copy(tmpFile, r.Body)
			if err == nil {
				r.Body.Close()
				r.Body, err = os.Open(tmpFile.Name())
			}
		}
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
	h.chanReqRsp <- reqRsp
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

var once sync.Once

func (h *hrq) Handle(method string, path string, handler http.HandlerFunc) {

	lock.Lock()
	defer lock.Unlock()
	handlerMap[method+"$"+path] = handler

	hl := func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		handler(w, r)
	}
	h.router.Handle(method, path, hl)
}

// GET is a shortcut for router.Handle(http.MethodGet, path, handle)
func (h *hrq) GET(path string, handle http.HandlerFunc) {
	h.Handle(http.MethodGet, path, handle)
}

// HEAD is a shortcut for router.Handle(http.MethodHead, path, handle)
func (h *hrq) HEAD(path string, handle http.HandlerFunc) {
	h.Handle(http.MethodHead, path, handle)
}

// OPTIONS is a shortcut for router.Handle(http.MethodOptions, path, handle)
func (h *hrq) OPTIONS(path string, handle http.HandlerFunc) {
	h.Handle(http.MethodOptions, path, handle)
}

// POST is a shortcut for router.Handle(http.MethodPost, path, handle)
func (h *hrq) POST(path string, handle http.HandlerFunc) {
	h.Handle(http.MethodPost, path, handle)
}

// PUT is a shortcut for router.Handle(http.MethodPut, path, handle)
func (h *hrq) PUT(path string, handle http.HandlerFunc) {
	h.Handle(http.MethodPut, path, handle)
}

// PATCH is a shortcut for router.Handle(http.MethodPatch, path, handle)
func (h *hrq) PATCH(path string, handle http.HandlerFunc) {
	h.Handle(http.MethodPatch, path, handle)
}

// DELETE is a shortcut for router.Handle(http.MethodDelete, path, handle)
func (h *hrq) DELETE(path string, handle http.HandlerFunc) {
	h.Handle(http.MethodDelete, path, handle)
}

func (h *hrq) Router() *httprouter.Router {
	return h.router
}

func (h *hrq) ApplyToGin(ginEngine *gin.Engine) {
	lock.RLock()
	defer lock.RUnlock()
	for k, _ := range handlerMap {
		method := strings.Split(k, "$")[0]
		path := strings.Split(k, "$")[1]
		ginEngine.Handle(method, path, func(c *gin.Context) {
			if handler, _, _ := h.router.Lookup(method, path); handler != nil {
				handler(c.Writer, c.Request, nil)
			} else {
				c.Writer.WriteHeader(http.StatusNotFound)
			}
		})
	}
}

func (h *hrq) ApplyToBeego(server *web.HttpServer) {
	lock.RLock()
	defer lock.RUnlock()
	for k, _ := range handlerMap {
		method := strings.Split(k, "$")[0]
		path := strings.Split(k, "$")[1]
		server.Handlers.AddMethod(method, path, func(ctx *beecontext.Context) {
			if handler, _, _ := h.router.Lookup(method, path); handler != nil {
				handler(ctx.ResponseWriter, ctx.Request, nil)
			} else {
				ctx.ResponseWriter.WriteHeader(http.StatusNotFound)
			}
		})
	}
}

func (h *hrq)ApplyToEcho(e *echo.Echo) {
	lock.RLock()
	defer lock.RUnlock()
	for k, _ := range handlerMap {
		method := strings.Split(k, "$")[0]
		path := strings.Split(k, "$")[1]
		e.Add(method, path, func(c echo.Context) error {
			if handler, _, _ := h.router.Lookup(method, path); handler != nil {
				handler(c.Response().Writer, c.Request(), nil)
			} else {
				c.Response().WriteHeader(http.StatusNotFound)
			}
			return nil
		})
	}
}

func (h *hrq) ApplyFromGin(ginEngine *gin.Engine) {
	for _, v := range ginEngine.Routes() {
		h.Handle(v.Method, v.Path, func(w http.ResponseWriter, r *http.Request) {
			ctx := &gin.Context{}
			r = r.WithContext(context.WithValue(r.Context(), hrqContextKey, ctx))
			ctx.Request = r
			ctx.Writer = &responseWriter{ResponseWriter: w}
			v.HandlerFunc(ctx)
		})
	}
}

func (h *hrq) MiddlewareForGin() gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.Request.WithContext(context.WithValue(c.Request.Context(), hrqContextKey, c))
		h.ServeHTTP(c.Writer, req)
	}
}

func (h *hrq) InstallFilterChanForBeego() {
	web.InsertFilterChain("/*", func(next web.FilterFunc) web.FilterFunc {
		return func(ctx *beecontext.Context) {
			c := context.WithValue(ctx.Request.Context(), hrqContextKey, ctx)
			c = context.WithValue(c, hrqFilterNext, next)
			ctx.Request = ctx.Request.WithContext(c)
			h.ServeHTTP(ctx.ResponseWriter, ctx.Request)
			next(ctx)
		}
	})
}

func (h *hrq) ListenAndServe(addr string) error {

	return http.ListenAndServe(addr, h.mux)
}

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
	lock.RLock()
	defer lock.RUnlock()
	for k, _ := range handlerMap {
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
	lock.RLock()
	defer lock.RUnlock()
	for k, _ := range handlerMap {
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
			ctx.SetRequest( ctx.Request().WithContext(c))
			ghrp.ServeHTTP(ctx.Response().Writer, ctx.Request())
			return nil
		}
	}
}

func ApplyToEcho(e *echo.Echo) {
	lock.RLock()
	defer lock.RUnlock()
	for k, _ := range handlerMap {
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
