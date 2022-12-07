package hrq

type stat struct {
	Path       string `json:"path"`
	Count      int64  `json:"count"`
	Avg        int64  `json:"avg"`
	Max        int64  `json:"max"`
	Min        int64  `json:"min"`
	TimeoutCnt int64  `json:"-"`
}

type reqTimeInfo struct {
	StartTime int64
	EndTime   int64
	Path      string
}

func (h *hrq) updateStat(ri reqTimeInfo) {
	ri.EndTime = ri.EndTime / 1000 / 1000
	ri.StartTime = ri.StartTime / 1000 / 1000
	h.statLock.Lock()
	defer h.statLock.Unlock()
	if _, ok := h.statMap[ri.Path]; !ok {
		h.statMap[ri.Path] = &stat{
			Path: ri.Path,
			Max:  ri.EndTime - ri.StartTime,
			Min:  ri.EndTime - ri.StartTime,
		}
	}
	s := h.statMap[ri.Path]
	s.Count++
	s.Avg = (s.Avg*(s.Count-1) + (ri.EndTime - ri.StartTime)) / s.Count
	if s.Max < (ri.EndTime - ri.StartTime) {
		s.Max = ri.EndTime - ri.StartTime
	}
	if s.Min > (ri.EndTime - ri.StartTime) {
		s.Min = ri.EndTime - ri.StartTime
	}
}

func (h *hrq) initStat() {
	go func() {
		for {
			select {
			case ri := <-h.chanReqInfo:
				h.updateStat(ri)
			}
		}
	}()
}

func (h *hrq) GetStat() map[string]*stat {
	h.statLock.Lock()
	defer h.statLock.Unlock()
	// copy
	statMap := make(map[string]*stat)
	for k, v := range h.statMap {
		statMap[k] = v
	}
	return statMap
}
