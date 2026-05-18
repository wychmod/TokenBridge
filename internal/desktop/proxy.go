package desktop

import "net/http"

type SPAProxy struct {
	router http.Handler
	ready  bool
}

func NewSPAProxy() *SPAProxy {
	return &SPAProxy{}
}

func (p *SPAProxy) SetRouter(router http.Handler) {
	p.router = router
	p.ready = true
}

// ServeHTTP delegates all requests to the shared router so desktop and browser modes behave consistently.
func (p *SPAProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !p.ready || p.router == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"server not ready"}`))
		return
	}
	p.router.ServeHTTP(w, r)
}
