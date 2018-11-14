package roshi

import "net/http"

type Middleware func(http.HandlerFunc) http.HandlerFunc
type Handler    func(w http.ResponseWriter, r *http.Request) (code int, err error)

type Server struct {
    middleware []Middleware
    log        Logger
	address    string
}

func NewServer(address string) *Server {
	return &Server{address: address}
}


func (s *Server) Start() {
	defer s.close()

	http.ListenAndServe(s.address, nil)
}


func (s *Server) ApplyMiddleware(middleware ...Middleware) {
	s.middleware = append(s.middleware, middleware...)
}


func (s *Server) ApplyLogger(logger Logger) {
	s.log = logger
}


func (s *Server) Route(path string, handler Handler) {
	http.Handle(path, s.wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code, err := handler(w, r)
		if nil != s.log {
			s.log.Access(r, int64(code), r.Response.ContentLength)

			if nil != err {
				s.log.Error(r, err)
			}
		}
	})))
}


func (s *Server) close() {
	if nil != s.log {
		s.log.Close()
	}
}


func (s *Server) wrap(handler http.Handler) http.Handler {
	if 0 == len(s.middleware) {
		return handler
	}

	for _, m := range s.middleware {
		handler = m(handler.ServeHTTP)
	}

	return handler
}