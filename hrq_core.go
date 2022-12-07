package hrq

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"
)

type reqrsq struct {
	uuid   string
	path   string
	method string
	begin  int64
	req    *http.Request
	rsp    http.ResponseWriter
	done   chan bool
}

type workerOption func(*worker)

func WithWorkerOption(workerCount int, maxQueue int) workerOption {
	return func(w *worker) {
		w.max = workerCount
		w.maxQueue = maxQueue
	}
}

type Config struct {
	// The number of goroutines that will be used to handle requests.
	// If <= 0, then the number of CPUs will be used.
	Workers int
	// The size of the queue that will be used to store requests.
	// If <= 0, then the default value will be used.
	MaxQueueSize int
	// TempDir is the directory to use for temporary files.
	TempDir string
	// TimeoutQueue is the maximum duration before timing out read of the request.
	// If TimeoutQueue is zero, no timeout is set.
	TimeoutQueue int64
	// TimeoutProcess is the maximum duration before timing out processing of the request.
	// If TimeoutProcess is zero, no timeout is set.
	TimeoutProcess int64

	PerRequestWorker int

	PerRequestQueue int

	// MaxConnection
	MaxConnection int
	// enable overload
	EnableOverload bool
	workerOptions  map[string]workerOption
}

func (c *Config) SetWorkerOption(method string, path string, option workerOption) *Config {
	key := method + "$" + path
	if _, ok := c.workerOptions[key]; !ok {
		c.workerOptions[key] = option
	}
	return c
}

type hrq struct {
	handlerLock   sync.RWMutex
	handlerMap    map[string]http.HandlerFunc
	once          sync.Once
	mux           *http.ServeMux
	chanReqRsp    chan *reqrsq
	router        *httprouter.Router
	config        *Config
	chanReqInfo   chan reqTimeInfo
	statLock      sync.Mutex
	statMap       map[string]*stat
	maxConn       chan struct{}
	mapWorker     map[string]*worker
	handlerWorker map[uintptr]*worker
}

const (
	noWritten        = -1
	defaultStatus    = http.StatusOK
	hrqContextKey    = "hrqContextKey"
	hrqFilterNextKey = "hrqFilterNextKey"
	hrqErrorKey      = "hrqErrorKey"
	hrqWorkerKey     = "hrqWorkerKey"
	hrqStartTimeKey  = "hrqStartTimeKey"
)

type httpError struct {
	Code     int         `json:"-"`
	Message  interface{} `json:"message"`
	Internal error       `json:"-"` // Stores the error returned by an external dependency
}

var (
	errUnsupportedMediaType        = newHTTPError(http.StatusUnsupportedMediaType)
	errNotFound                    = newHTTPError(http.StatusNotFound)
	errUnauthorized                = newHTTPError(http.StatusUnauthorized)
	errForbidden                   = newHTTPError(http.StatusForbidden)
	errMethodNotAllowed            = newHTTPError(http.StatusMethodNotAllowed)
	errStatusRequestEntityTooLarge = newHTTPError(http.StatusRequestEntityTooLarge)
	errTooManyRequests             = newHTTPError(http.StatusTooManyRequests)
	errBadRequest                  = newHTTPError(http.StatusBadRequest)
	errBadGateway                  = newHTTPError(http.StatusBadGateway)
	errInternalServerError         = newHTTPError(http.StatusInternalServerError)
	errRequestTimeout              = newHTTPError(http.StatusRequestTimeout)
	errServiceUnavailable          = newHTTPError(http.StatusServiceUnavailable)
	errServerOverloaded            = newHTTPError(http.StatusServiceUnavailable, "server overloaded")
	errValidatorNotRegistered      = errors.New("validator not registered")
	errRendererNotRegistered       = errors.New("renderer not registered")
	errInvalidRedirectCode         = errors.New("invalid redirect status code")
	errCookieNotFound              = errors.New("cookie not found")
	errInvalidCertOrKeyType        = errors.New("invalid cert or key type, must be string or []byte")
	errInvalidListenerNetwork      = errors.New("invalid listener network")
)

// Error makes it compatible with `error` interface.
func (he *httpError) Error() string {
	if he.Internal == nil {
		return fmt.Sprintf("code=%d, message=%v", he.Code, he.Message)
	}
	return fmt.Sprintf("code=%d, message=%v, internal=%v", he.Code, he.Message, he.Internal)
}
func (he *httpError) String() string {
	buff, err := json.Marshal(he)
	if err != nil {
		return err.Error()
	}
	return string(buff)
}

func newHTTPError(code int, message ...interface{}) *httpError {
	he := &httpError{Code: code, Message: http.StatusText(code)}
	if len(message) > 0 {
		he.Message = message[0]
	}
	return he
}

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
	if conf.Workers <= 0 {
		conf.Workers = runtime.NumCPU() * 10
	}
	if conf.MaxQueueSize <= 0 {
		conf.MaxQueueSize = runtime.NumCPU() * 100
	}
	if conf.MaxConnection <= 0 {
		conf.MaxConnection = 1000
	}
	h := &hrq{
		mux:           http.NewServeMux(),
		chanReqRsp:    make(chan *reqrsq, conf.MaxQueueSize),
		config:        conf,
		once:          sync.Once{},
		router:        httprouter.New(),
		statMap:       make(map[string]*stat),
		chanReqInfo:   make(chan reqTimeInfo, 10000),
		handlerMap:    make(map[string]http.HandlerFunc),
		maxConn:       make(chan struct{}, conf.MaxConnection),
		mapWorker:     make(map[string]*worker, conf.Workers),
		handlerWorker: make(map[uintptr]*worker, conf.Workers),
	}
	h.mux.HandleFunc("/", h.ServeHTTP)
	return h
}

func (h *hrq) initHrq(worker int, queueSize int) {
	// http queue consumer
	go h.initStat()
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
						if reqRsp == nil {
							continue
						}
						hrqCxt := reqRsp.req.Context().Value(hrqContextKey)
						if hrqCxt == nil {
							hrqCxt = reqRsp.req.Context()
						}
						if h.checkOverLoad(reqRsp.rsp, reqRsp.req) {
							reqRsp.done <- true
							continue
						}
						apt := h.getAdapter(hrqCxt, reqRsp, h)
						err := apt.Next()
						if err != nil {
							reqRsp.rsp.WriteHeader(err.Code)
							reqRsp.rsp.Write([]byte(err.Message.(string)))
						}
						reqRsp.done <- true

					}

				}
			}
			for {
				handler()
			}
		}()

	}
}

func (h *hrq) checkOverLoad(w http.ResponseWriter, r *http.Request) bool {

	startTime := r.Context().Value(hrqStartTimeKey)
	if startTime != nil {
		if time.Now().UnixNano()-startTime.(int64) > h.config.TimeoutQueue*1000*1000 {
			r = r.WithContext(context.WithValue(r.Context(), hrqErrorKey, errServerOverloaded))
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(errServerOverloaded.String()))
			return true
		}
	} else {
		if h.config.MaxConnection > 0 {
			select {
			case h.maxConn <- struct{}{}:
				return false
			default:
				r = r.WithContext(context.WithValue(r.Context(), hrqErrorKey, errServerOverloaded))
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte(errServerOverloaded.String()))
				return true
			}
		}
	}
	return false

}

// define default http handler
func (h *hrq) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.checkOverLoad(w, r) {
		return
	}
	defer func() {
		<-h.maxConn
	}()
	h.once.Do(func() {
		h.initHrq(h.config.Workers, h.config.MaxQueueSize)
	})
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
	startTime := time.Now().UnixNano()
	r = r.WithContext(context.WithValue(r.Context(), hrqStartTimeKey, startTime))
	reqRsp := &reqrsq{
		begin:  startTime,
		uuid:   "uuid",
		path:   r.URL.Path,
		method: r.Method,
		req:    r,
		rsp:    w,
		done:   c,
	}
	ri := reqTimeInfo{
		StartTime: startTime,
		Path:      r.URL.Path,
	}
	//if h.config.TimeoutProcess > 0 {
	//	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(h.config.TimeoutProcess)*time.Millisecond)
	//	reqRsp.req = r.WithContext(ctx)
	//	defer cancel()
	//}

	//key := reqRsp.method + "$" + reqRsp.path
	//if v, ok := h.mapWorker[key]; ok {
	//	v.submitReq(reqRsp)
	//} else {
	//	h.chanReqRsp <- reqRsp
	//}

	//get worker from context

	if handler, _, _ := h.router.Lookup(r.Method, r.URL.Path); handler != nil {
		ptr := reflect.ValueOf(handler).Pointer()
		if v, ok := h.handlerWorker[ptr]; ok {
			v.submitReq(reqRsp)
		} else {
			h.chanReqRsp <- reqRsp
		}
	} else {
		h.chanReqRsp <- reqRsp
	}
	//h.chanReqRsp <- reqRsp

	//h.chanReqRsp <- reqRsp
	<-reqRsp.done
	ri.EndTime = time.Now().UnixNano()
	h.chanReqInfo <- ri

	//close(c)
}

func (h *hrq) Handle(method string, path string, handler http.HandlerFunc, options ...workerOption) {

	h.handlerLock.Lock()
	defer h.handlerLock.Unlock()
	key := method + "$" + path
	h.handlerMap[key] = handler

	if _, ok := h.mapWorker[key]; !ok {
		var w *worker
		if len(options) > 0 {
			w = newWorker2()
			w.apply(options...)
			w.hrq = h
			w.reqChan = make(chan *reqrsq, w.maxQueue)
		} else {
			if o,ok:= h.config.workerOptions[key];ok {
				w = newWorker2()
				w.apply(o)
				w.hrq = h
				w.reqChan = make(chan *reqrsq, w.maxQueue)
			} else {
				w = newWorker(h.config.PerRequestWorker, h.config.PerRequestQueue, h)
			}
		}
		h.mapWorker[key] = w
		w.start()
	}

	hl := func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		handler(w, r)
	}
	h.handlerWorker[reflect.ValueOf(hl).Pointer()] = h.mapWorker[key]
	h.router.Handle(method, path, hl)
}

// GET is a shortcut for router.Handle(http.MethodGet, path, handle)
func (h *hrq) GET(path string, handle http.HandlerFunc, options ...workerOption) {
	h.Handle(http.MethodGet, path, handle, options...)
}

// HEAD is a shortcut for router.Handle(http.MethodHead, path, handle)
func (h *hrq) HEAD(path string, handle http.HandlerFunc, options ...workerOption) {
	h.Handle(http.MethodHead, path, handle, options...)
}

// OPTIONS is a shortcut for router.Handle(http.MethodOptions, path, handle)
func (h *hrq) OPTIONS(path string, handle http.HandlerFunc, options ...workerOption) {
	h.Handle(http.MethodOptions, path, handle, options...)
}

// POST is a shortcut for router.Handle(http.MethodPost, path, handle)
func (h *hrq) POST(path string, handle http.HandlerFunc, options ...workerOption) {
	h.Handle(http.MethodPost, path, handle, options...)
}

// PUT is a shortcut for router.Handle(http.MethodPut, path, handle)
func (h *hrq) PUT(path string, handle http.HandlerFunc, options ...workerOption) {
	h.Handle(http.MethodPut, path, handle, options...)
}

// PATCH is a shortcut for router.Handle(http.MethodPatch, path, handle)
func (h *hrq) PATCH(path string, handle http.HandlerFunc, options ...workerOption) {
	h.Handle(http.MethodPatch, path, handle, options...)
}

// DELETE is a shortcut for router.Handle(http.MethodDelete, path, handle)
func (h *hrq) DELETE(path string, handle http.HandlerFunc, options ...workerOption) {
	h.Handle(http.MethodDelete, path, handle, options...)
}

func (h *hrq) Router() *httprouter.Router {
	return h.router
}

func (h *hrq) ApplyToGin(ginEngine *gin.Engine) {
	h.handlerLock.RLock()
	defer h.handlerLock.RUnlock()
	for k, _ := range h.handlerMap {
		method := strings.Split(k, "$")[0]
		path := strings.Split(k, "$")[1]
		ginEngine.Handle(method, path, func(c *gin.Context) {
			if handler, _, _ := h.router.Lookup(method, path); handler != nil {
				handler(c.Writer, c.Request,nil)
			} else {
				c.Writer.WriteHeader(http.StatusNotFound)
			}
		})
	}
}

func (h *hrq) ApplyToBeego(server *web.HttpServer) {
	h.handlerLock.RLock()
	defer h.handlerLock.RUnlock()
	for k, _ := range h.handlerMap {
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

func (h *hrq) ApplyToEcho(e *echo.Echo) {
	h.handlerLock.RLock()
	defer h.handlerLock.RUnlock()
	for k, _ := range h.handlerMap {
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

func (h *hrq) MiddlewareForEcho() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			c := context.WithValue(ctx.Request().Context(), hrqContextKey, ctx)
			c = context.WithValue(c, hrqFilterNextKey, next)
			ctx.SetRequest(ctx.Request().WithContext(c))
			ghrp.ServeHTTP(ctx.Response().Writer, ctx.Request())
			if err := ctx.Request().Context().Value(hrqErrorKey); err != nil {
				ctx.Error(err.(error))
				return err.(error)
			}
			return nil
		}
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
		if err := c.Request.Context().Value(hrqErrorKey); err != nil {
			c.Abort()
			return
		}
	}
}

func (h *hrq) InstallFilterChanForBeego() {
	web.InsertFilterChain("*", func(next web.FilterFunc) web.FilterFunc {
		return func(ctx *beecontext.Context) {
			c := context.WithValue(ctx.Request.Context(), hrqContextKey, ctx)
			c = context.WithValue(c, hrqFilterNextKey, next)
			ctx.Request = ctx.Request.WithContext(c)
			h.ServeHTTP(ctx.ResponseWriter, ctx.Request)
			if err := ctx.Request.Context().Value(hrqErrorKey); err != nil {
				e := err.(*httpError)
				ctx.Abort(e.Code, e.String())
			}
		}
	})
}

func (h *hrq) ListenAndServe(addr string) error {

	return http.ListenAndServe(addr, h.mux)
}
