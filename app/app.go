package app

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/SlyMarbo/rss"
	"github.com/kvch/redme/model"
)

type ReqHandler func(http.ResponseWriter, *http.Request) error

var (
	db        *model.RedMeDB
	templates map[string]*template.Template
)

type PostsPage struct {
	Posts   []*rss.Item
	Err     string
	Success string
}

type FeedsPage struct {
	Feeds   []*model.RedMeFeed
	Err     string
	Success string
}

func (fn ReqHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := fn(w, r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func InitializeFedMe(path string) {
	var err error
	db, err = model.NewRedMeDBConn(path)
	if err != nil {
		log.Fatalln("Error while connecting to DB:", err)
	}

	initTemplates()
}

func initTemplates() {
	if templates == nil {
		templates = make(map[string]*template.Template)
	}
	templatesDir := "templates/"
	basePath := "templates/base.tmpl"
	successPath := "templates/success.tmpl"
	errorPath := "templates/error.tmpl"
	layouts, err := filepath.Glob(templatesDir + "*.tmpl")
	if err != nil {
		log.Fatal("Error while initializing templates:", err)
	}

	funcMap := template.FuncMap{
		"noescape": func(s string) template.HTML {
			return template.HTML(s)
		},
	}

	for _, layout := range layouts {
		if layout != basePath && layout != successPath && layout != errorPath {
			templates[filepath.Base(layout)] = template.Must(template.New("").Funcs(funcMap).ParseFiles(layout, basePath, errorPath, successPath))
		}
	}
}

func ShowUnreadPosts(w http.ResponseWriter, r *http.Request) error {
	posts, err := db.GetAllUnreadPosts()
	if err != nil {
		p := &PostsPage{Posts: nil, Success: "", Err: "Error while fetching posts"}
		return renderTemplate(w, "index.tmpl", p)
	}
	p := &PostsPage{Posts: posts, Success: "", Err: ""}
	return renderTemplate(w, "index.tmpl", p)
}

func MarkAllPostsRead(w http.ResponseWriter, r *http.Request) error {
	err := db.MarkAllPostsRead()
	if err != nil {
		posts, _ := db.GetAllUnreadPosts()
		p := &PostsPage{Posts: posts, Success: "", Err: "Error while marking posts as read"}
		return renderTemplate(w, "index.tmpl", p)
	}

	p := &PostsPage{Posts: nil, Success: "", Err: ""}
	return renderTemplate(w, "index.tmpl", p)
}

func RefreshFeeds(w http.ResponseWriter, r *http.Request) error {
	feeds, err := db.GetAllFeeds()
	if err != nil {
		log.Println(err)
		p := &PostsPage{Posts: nil, Success: "", Err: "Error while fetching feeds from db"}
		return renderTemplate(w, "index.tmpl", p)
	}

	for _, f := range feeds {
		err := f.Feed.Update()
		if err != nil {
			log.Println("Error while updating feed", f.Feed.Title, "(", f.Feed.UpdateURL, ")", err.Error())
			posts, _ := db.GetAllUnreadPosts()
			p := &PostsPage{Posts: posts, Success: "", Err: "Error while updating feed"}
			return renderTemplate(w, "index.tmpl", p)
		}
		for _, i := range f.Feed.Items {
			db.AddPost(f, i)
		}
	}
	posts, _ := db.GetAllUnreadPosts()
	p := &PostsPage{Posts: posts, Success: "", Err: ""}
	return renderTemplate(w, "index.tmpl", p)
}

func ListFeeds(w http.ResponseWriter, r *http.Request) error {
	feeds, err := db.GetAllFeeds()
	if err != nil {
		p := &FeedsPage{Feeds: nil, Success: "", Err: "Error while fetching feeds from db"}
		return renderTemplate(w, "add.tmpl", p)
	}
	p := &FeedsPage{Feeds: feeds, Success: "", Err: ""}
	return renderTemplate(w, "add.tmpl", p)
}

func AddFeed(w http.ResponseWriter, r *http.Request) error {
	r.ParseForm()

	var filters []string
	if (r.Form.Get("filters")) != "" {
		filters = strings.Split(r.Form.Get("filters"), ",")
	}
	newFeed, err := model.NewRedMeFeed(r.Form.Get("feed"), filters)
	if err != nil {
		log.Println(err)
		f, _ := db.GetAllFeeds()
		p := &FeedsPage{Feeds: f, Success: "", Err: "Error while adding feed"}
		return renderTemplate(w, "add.tmpl", p)
	}

	err = db.AddFeed(newFeed)
	if err != nil {
		log.Println(err)
		f, _ := db.GetAllFeeds()
		p := &FeedsPage{Feeds: f, Success: "", Err: "Error while saving feed to db"}
		return renderTemplate(w, "add.tmpl", p)
	}

	f, _ := db.GetAllFeeds()
	p := &FeedsPage{Feeds: f, Success: "Successfully added feed", Err: ""}
	return renderTemplate(w, "add.tmpl", p)
}

func renderTemplate(w http.ResponseWriter, name string, data interface{}) error {
	tmpl, ok := templates[name]
	if !ok {
		log.Fatal("Template does not exist:", name)
	}
	return tmpl.ExecuteTemplate(w, "base", data)
}
