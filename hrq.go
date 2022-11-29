package hrq

import (
	"github.com/gin-gonic/gin"
	"github.com/julienschmidt/httprouter"
	"log"
	"net/http"
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

func init() {
	// http queue consumer
	for i := 0; i < 1000; i++ {
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
	//ginEngine.Any("/", func(c *gin.Context) {
	//	ServeHTTP(c.Writer, c.Request)
	//})
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

//func MiddlewareForGin() gin.HandlerFunc {
//	return func(c *gin.Context) {
//		serveHTTP := func(w http.ResponseWriter, r *http.Request) {
//
//		}
//		serveHTTP(c.Writer, c.Request)
//
//	}
//}

func MiddlewareForGin() gin.HandlerFunc {
	return func(c *gin.Context) {
		ServeHTTP(c.Writer, c.Request)
	}
}

func ListenAndServe(addr string) error {

	return http.ListenAndServe(addr, mux)
}
