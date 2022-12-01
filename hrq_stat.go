package hrq

type stat struct {
	Path       string  `json:"path"`
	Count      int64   `json:"count"`
	Avg        float32 `json:"avg"`
	Max        float32 `json:"max"`
	Min        float32 `json:"min"`
	TimeoutCnt int64   `json:"-"`
}

type reqTimeInfo struct {
	StartTime int64
	EndTime   int64
	Path      string
}

func (h *hrq) updateStat(ri reqTimeInfo) {
	h.statLock.Lock()
	defer h.statLock.Unlock()
	if _, ok := h.statMap[ri.Path]; !ok {
		h.statMap[ri.Path] = &stat{
			Path: ri.Path,
			Max:  float32(ri.EndTime - ri.StartTime),
			Min:  float32(ri.EndTime - ri.StartTime),
		}
	}
	s := h.statMap[ri.Path]
	s.Count++
	s.Avg = (s.Avg*float32(s.Count-1) + float32(ri.EndTime-ri.StartTime)) / float32(s.Count)
	if s.Max < float32(ri.EndTime-ri.StartTime) {
		s.Max = float32(ri.EndTime - ri.StartTime)
	}
	if s.Min > float32(ri.EndTime-ri.StartTime) {
		s.Min = float32(ri.EndTime - ri.StartTime)
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
