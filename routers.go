package main

import (
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
)

func Router4pda(provider *RSSProvider) http.Handler {
	router := &router4pda{
		provider: provider,
	}
	r := chi.NewRouter()
	r.Get("/{topicID}", router.topicRSS)
	return r
}

type router4pda struct {
	provider *RSSProvider
}

func (rp *router4pda) topicRSS(w http.ResponseWriter, r *http.Request) {
	topicID, err := strconv.Atoi(chi.URLParam(r, "topicID"))
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	feed, err := rp.provider.Feed(topicID)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	rss, err := feed.ToRss()
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	_, _ = w.Write([]byte(rss))
}
