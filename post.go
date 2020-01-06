package main

import (
	"time"
)

type Post struct {
	ID       int64
	Created  time.Time
	Updated  time.Time
	HTML     string
	MainText string
	Link     string
}
