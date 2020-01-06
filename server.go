package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

type Server struct {
	port int
}

func NewServer(port int) *Server {
	return &Server{
		port: port,
	}
}

func (s *Server) Start() {
	r := chi.NewRouter()
	s.setupMiddlewares(r)

	rssProvider := NewRSSProvider()

	r.Route("/rss", func(r chi.Router) {
		r.Mount("/4pda", Router4pda(rssProvider))
	})

	log.Printf("started on :%d\n", s.port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", s.port), r)
	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) setupMiddlewares(r *chi.Mux) {
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
}
