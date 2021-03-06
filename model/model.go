package model

import (
	"database/sql"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/SlyMarbo/rss"
	_ "github.com/mattn/go-sqlite3"
)

const (
	sqlCreateFeed = `CREATE TABLE IF NOT EXISTS feed(
	id      INTEGER PRIMARY KEY,
	title   TEXT NOT NULL,
	url     TEXT NOT NULL,
	filters TEXT,
	UNIQUE (url, filters));`
	sqlCreatePost = `CREATE TABLE IF NOT EXISTS post(
	id      INTEGER PRIMARY KEY,
	feed    INTEGER,
	url     TEXT NOT NULL,
	title   TEXT NOT NULL,
	summary TEXT,
	content TEXT,
	read    INTEGER,
	date    DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(feed) REFERENCES feed(id),
	UNIQUE (url) ON CONFLICT IGNORE
	);`
	sqlNewFeed        = `INSERT INTO feed(url, title, filters) VALUES(?, ?, ?);`
	sqlNewPost        = `INSERT INTO post(url, title, summary, content, read, feed) VALUES(?, ?, ?, ?, 0, ?);`
	sqlGetUnreadPosts = `SELECT post.id, post.url, post.title, post.summary, post.content, post.date, feed.title
	FROM post, feed WHERE read = 0 AND feed.id=post.feed ORDER BY post.date DESC, post.id ASC;`
	sqlGetAllFeeds = `SELECT id, title, url, filters FROM feed;`
	sqlMarkAllRead = `UPDATE post SET read = 1 WHERE read = 0 AND id <= ?;`
)

type RedMeFeed struct {
	id      int64
	Filters []string
	Feed    *rss.Feed
}

type RedMePost struct {
	Id        int
	Item      *rss.Item
	FeedTitle string
	Date      string
}

func NewRedMeFeed(url string, filters []string) (*RedMeFeed, error) {
	feed := new(RedMeFeed)
	openedFeed, err := rss.Fetch(url)
	if err != nil {
		return nil, err
	}
	feed.Feed = openedFeed

	feed.Filters = filters
	return feed, nil
}

type RedMeDB struct {
	sync.RWMutex
	db *sql.DB
}

func NewRedMeDBConn(path string) (*RedMeDB, error) {
	log.Println("Connecting to DB at", path)

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(sqlCreateFeed)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(sqlCreatePost)
	if err != nil {
		return nil, err
	}

	log.Println("DB connection created")

	return &RedMeDB{db: db}, nil
}

func (r *RedMeDB) AddFeed(f *RedMeFeed) error {
	var filters string
	if f.Filters == nil {
		filters = ""
	} else {
		filters = strings.Join(f.Filters, ",")
	}

	r.Lock()
	defer r.Unlock()
	res, err := r.db.Exec(sqlNewFeed, f.Feed.UpdateURL, f.Feed.Title, filters)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	f.id = id

	return nil
}

func (r *RedMeDB) AddPost(f *RedMeFeed, i *rss.Item) error {
	var err error
	if isAddableItem(i, f.Filters) {
		r.Lock()
		_, err = r.db.Exec(sqlNewPost, i.Link, i.Title, i.Summary, i.Content, f.id)
		r.Unlock()
	}

	return err
}

func isAddableItem(i *rss.Item, filters []string) bool {
	if filters == nil {
		return true
	}

	for _, filter := range filters {
		if strings.Contains(strings.ToUpper(i.Title), strings.ToUpper(filter)) && !i.Read {
			return true
		}
	}
	return false
}

func (r *RedMeDB) GetAllUnreadPosts() ([]*RedMePost, error) {
	rows, err := r.db.Query(sqlGetUnreadPosts)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	l := make([]*RedMePost, 0)
	i := new(RedMePost)
	d := new(time.Time)
	i.Item = new(rss.Item)
	for rows.Next() {
		err = rows.Scan(&i.Id, &i.Item.Link, &i.Item.Title, &i.Item.Summary, &i.Item.Content, &d, &i.FeedTitle)
		if err != nil {
			return nil, err
		}
		i.Date = d.Format("2006 Jan 2 15:04")
		l = append(l, i)
		i = new(RedMePost)
		i.Item = new(rss.Item)
		d = new(time.Time)
	}

	return l, nil
}

func (r *RedMeDB) GetAllFeeds() ([]*RedMeFeed, error) {
	rows, err := r.db.Query(sqlGetAllFeeds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	f := make([]*RedMeFeed, 0)
	e := new(RedMeFeed)
	e.Feed = new(rss.Feed)
	var rawFilters string

	for rows.Next() {
		err = rows.Scan(&e.id, &e.Feed.Title, &e.Feed.UpdateURL, &rawFilters)
		e.Filters = nil
		if rawFilters != "" {
			e.Filters = strings.Split(",", rawFilters)
		}

		if err != nil {
			return nil, err
		}
		f = append(f, e)
		e = new(RedMeFeed)
		e.Feed = new(rss.Feed)
	}
	return f, nil

}

func (r *RedMeDB) MarkAllPostsRead(id string) error {
	r.Lock()
	_, err := r.db.Exec(sqlMarkAllRead, id)
	r.Unlock()
	return err
}
