package hrq

import "log"

type worker struct {
	max      int
	maxQueue int
	reqChan  chan *reqrsq
	hrq      *hrq
}

func newWorker(max int, maxQueue int, hrq *hrq) *worker {
	w := &worker{
		max:      max,
		maxQueue: maxQueue,
		reqChan:  make(chan *reqrsq, maxQueue),
		hrq:      hrq,
	}
	w.start()
	return w
}

func (w *worker) submitReq(req *reqrsq) {
	w.reqChan <- req
}

// start worker
func (w *worker) start() {
	for i := 0; i < w.max; i++ {
		go w.run()
	}
}

func (w *worker) run() {

	inner := func() {
		// panic recover
		defer func() {
			if err := recover(); err != nil {
				log.Println(err)
			}
		}()
		for {
			select {
			case reqRsp := <-w.reqChan:
				if reqRsp == nil {
					continue
				}
				hrqCxt := reqRsp.req.Context().Value(hrqContextKey)
				if hrqCxt == nil {
					hrqCxt = reqRsp.req.Context()
				}
				apt := w.hrq.getAdapter(hrqCxt, reqRsp, w.hrq)
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
		inner()
	}
}
