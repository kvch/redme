package main

import (
	"flag"
	"net/http"

	"github.com/kvch/redme/app"
)

func main() {
	dbURI := flag.String("db", ":memory:", "Path to the database")
	addr := flag.String("address", ":8088", "Address of webserver")
	flag.Parse()

	app.InitializeFedMe(*dbURI)

	http.Handle("/", app.ReqHandler(app.ShowUnreadPosts))
	http.Handle("/show", app.ReqHandler(app.ListFeeds))
	http.Handle("/add", app.ReqHandler(app.AddFeed))
	http.Handle("/refresh", app.ReqHandler(app.RefreshFeeds))
	http.Handle("/allread", app.ReqHandler(app.MarkAllPostsRead))
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	http.ListenAndServe(*addr, nil)
}
