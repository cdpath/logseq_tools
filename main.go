// This program requires the FTS5 extension for SQLite.
// Use the following command to build the program: go build -tags 'fts5'

package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-sqlite3"
	_ "github.com/mattn/go-sqlite3"
)

type Page struct {
	ID           int                     `json:"id"`
	CreatedAt    int64                   `json:"createdAt"`
	UpdatedAt    int64                   `json:"updatedAt"`
	UUID         string                  `json:"uuid"`
	Journal      bool                    `json:"journal?"`
	OriginalName string                  `json:"originalName"`
	Properties   *map[string]interface{} `json:"properties,omitempty"`
	Tags         string                  `json:"-"`
}

type OutputItem struct {
	UID      string      `json:"uid"`
	Title    string      `json:"title"`
	Subtitle string      `json:"subtitle"`
	Arg      interface{} `json:"arg"`
	Icon     string      `json:"icon"`
}

func main() {
	// Define subcommands
	buildCmd := flag.NewFlagSet("build", flag.ExitOnError)
	queryCmd := flag.NewFlagSet("query", flag.ExitOnError)
	tagCmd := flag.NewFlagSet("tag", flag.ExitOnError)
	tagsCmd := flag.NewFlagSet("tags", flag.ExitOnError) // Add this line

	// Parse the command line arguments
	if len(os.Args) < 2 {
		fmt.Println("Please specify a subcommand: build, query, or tag")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "build":
		buildCmd.Parse(os.Args[2:])
		pages := fetchPages()

		if _, err := os.Stat("pages.db"); !os.IsNotExist(err) {
			err := os.Remove("pages.db") // Remove the existing pages.db file if it exists
			if err != nil {
				log.Fatalf("Error removing pages.db: %v", err)
			}
		}

		db := initDB("pages.db")
		defer db.Close()
		createTable(db)
		createFTSTable(db)
		insertPages(db, pages)

	case "query":
		queryCmd.Parse(os.Args[2:])
		if len(queryCmd.Args()) == 0 {
			fmt.Println("Please provide a query after the 'query' subcommand")
			os.Exit(1)
		}
		queryArg := queryCmd.Arg(0)
		db := initDB("pages.db")
		defer db.Close()
		results := searchPages(db, queryArg)

		outputItems := makeOutputItems(results)
		output := map[string][]OutputItem{"items": outputItems}
		jsonOutput, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(jsonOutput))

	case "tag":
		tagCmd.Parse(os.Args[2:])
		if tagCmd.NArg() < 1 {
			fmt.Println("Please provide at least one tag after the 'tag' subcommand")
			os.Exit(1)
		}
		tags := tagCmd.Args()
		db := initDB("pages.db")
		defer db.Close()
		results := filterPagesByTags(db, tags)

		outputItems := makeOutputItems(results)
		output := map[string][]OutputItem{"items": outputItems}
		jsonOutput, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(jsonOutput))

	case "tags":
		tagsCmd.Parse(os.Args[2:])
		var tagFilter string
		if len(tagsCmd.Args()) > 0 {
			tagFilter = tagsCmd.Arg(0)
		} else {
			tagFilter = ""
		}
		db := initDB("pages.db")
		defer db.Close()
		printTags(db, tagFilter)

	default:
		fmt.Printf("%q is not a valid subcommand\n", os.Args[1])
		os.Exit(1)
	}
}

func makeOutputItems(results []Page) []OutputItem {
	outputItems := make([]OutputItem, len(results))
	logseqGraph := os.Getenv("LogseqGraph")
	if logseqGraph == "" {
		logseqGraph = "Logseq"
	}

	for i, result := range results {
		encodedOriginalName := url.QueryEscape(result.OriginalName)

		outputItems[i] = OutputItem{
			UID:      result.UUID,
			Title:    result.OriginalName,
			Subtitle: result.Tags,
			Arg:      fmt.Sprintf("logseq://graph/%s?page=%s", logseqGraph, encodedOriginalName),
			Icon:     "icon.png",
		}
	}
	return outputItems
}

func fetchPages() []Page {
	url := "http://127.0.0.1:12315/api"
	req, err := http.NewRequest("POST", url, strings.NewReader(`{"method": "logseq.Editor.getAllPages"}`))
	if err != nil {
		log.Fatal(err)
	}

	logseqToken := os.Getenv("LogseqToken")
	if logseqToken == "" {
		log.Fatal("LogseqToken environment variable is not set")
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", logseqToken))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	var pages []Page
	err = json.NewDecoder(resp.Body).Decode(&pages)
	if err != nil {
		log.Fatal(err)
	}

	return pages
}

func initDB(filepath string) *sql.DB {
	sql.Register("sqlite3_simple",
		&sqlite3.SQLiteDriver{
			Extensions: []string{
				"./libsimple",
			},
		})
	db, err := sql.Open("sqlite3_simple", filepath+"?_load_extension=1")
	if err != nil {
		log.Fatalf("Error opening the database: %v", err)
	}
	if db == nil {
		log.Fatal("db nil")
	}

	// Add this line to print more details about the error
	// db.Exec("PRAGMA journal_mode=WAL") // This line enables Write-Ahead Logging, which might help with some issues

	return db
}

func createTable(db *sql.DB) {
	sql := `
	CREATE TABLE IF NOT EXISTS pages (
		id INTEGER NOT NULL PRIMARY KEY,
		created_at INTEGER,
		updated_at INTEGER,
		uuid TEXT,
		journal BOOLEAN,
		original_name TEXT,
		properties TEXT
	);

	CREATE TABLE IF NOT EXISTS tags (
		id INTEGER NOT NULL PRIMARY KEY,
		page_id INTEGER,
		tag TEXT,
		FOREIGN KEY (page_id) REFERENCES pages (id) ON DELETE CASCADE
	);
	`

	_, err := db.Exec(sql)
	if err != nil {
		log.Fatalf("Error creating table: %v, %v", sql, err)
	}
}

func createFTSTable(db *sql.DB) {
	sql := `
	CREATE VIRTUAL TABLE IF NOT EXISTS pages_fts USING fts5(original_name, tokenize = 'simple');
	`

	_, err := db.Exec(sql)
	if err != nil {
		log.Fatalf("Error creating FTS table: %v, %v", sql, err)
	}
}

func insertPages(db *sql.DB, pages []Page) {
	insertPageSQL := `
	INSERT OR REPLACE INTO pages (id, created_at, updated_at, uuid, journal, original_name, properties)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	insertTagSQL := `
	INSERT INTO tags (page_id, tag)
	VALUES (?, ?)
	`

	insertFTSSQL := `
	INSERT OR REPLACE INTO pages_fts (rowid, original_name)
	VALUES (?, ?)
	`

	for _, page := range pages {
		propertiesJSON, _ := json.Marshal(page.Properties)

		result, err := db.Exec(insertPageSQL, page.ID, page.CreatedAt, page.UpdatedAt, page.UUID, page.Journal, page.OriginalName, string(propertiesJSON))
		if err != nil {
			log.Fatal(err)
		}

		rowID, err := result.LastInsertId()
		if err != nil {
			log.Fatal(err)
		}

		if page.Properties != nil {
			if tags, ok := (*page.Properties)["tags"].([]interface{}); ok {
				for _, tag := range tags {
					tagStr := fmt.Sprintf("%v", tag)
					_, err = db.Exec(insertTagSQL, rowID, tagStr)
					if err != nil {
						log.Fatal(err)
					}
				}
			}
		}

		_, err = db.Exec(insertFTSSQL, rowID, page.OriginalName)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func searchPages(db *sql.DB, query string) []Page {
	rows, err := db.Query(`
		SELECT pages.id, pages.created_at, pages.updated_at, pages.uuid, pages.journal, pages.original_name, pages.properties, IFNULL(GROUP_CONCAT(tags.tag, ' '), '') as tags
		FROM pages
		LEFT JOIN tags ON pages.id = tags.page_id
		WHERE pages.rowid IN (SELECT rowid FROM pages_fts WHERE original_name MATCH ?)
		GROUP BY pages.id
	`, query)

	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var pages []Page
	for rows.Next() {
		var page Page
		var propertiesStr string
		err = rows.Scan(&page.ID, &page.CreatedAt, &page.UpdatedAt, &page.UUID, &page.Journal, &page.OriginalName, &propertiesStr, &page.Tags)
		if err != nil {
			log.Fatal(err)
		}

		if propertiesStr != "" {
			var properties map[string]interface{}
			err = json.Unmarshal([]byte(propertiesStr), &properties)
			if err != nil {
				log.Fatal(err)
			}
			page.Properties = &properties
		}

		pages = append(pages, page)
	}
	return pages
}

func filterPagesByTags(db *sql.DB, tags []string) []Page {
	query := `
	SELECT pages.id, pages.created_at, pages.updated_at, pages.uuid, pages.journal, pages.original_name, pages.properties, GROUP_CONCAT(tags.tag, ' ') AS tags
	FROM pages
	INNER JOIN tags ON pages.id = tags.page_id
	WHERE tags.tag IN (` + strings.Repeat("?, ", len(tags)-1) + `?)
	GROUP BY pages.id
	HAVING COUNT(DISTINCT tags.tag) = ?
	`

	args := make([]interface{}, len(tags)+1)
	for i, tag := range tags {
		args[i] = tag
	}
	args[len(tags)] = len(tags)

	log.Printf("sql=%v, args=%v", query, args)

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var pages []Page
	for rows.Next() {
		var page Page
		var propertiesStr string
		err = rows.Scan(&page.ID, &page.CreatedAt, &page.UpdatedAt, &page.UUID, &page.Journal, &page.OriginalName, &propertiesStr, &page.Tags)
		if err != nil {
			log.Fatal(err)
		}

		if propertiesStr != "" {
			var properties map[string]interface{}
			err = json.Unmarshal([]byte(propertiesStr), &properties)
			if err != nil {
				log.Fatal(err)
			}
			page.Properties = &properties
		}

		pages = append(pages, page)
	}
	return pages
}

// tags subcommand

func printTags(db *sql.DB, tagFilter string) {
	var rows *sql.Rows
	var err error

	if tagFilter == "" {
		rows, err = db.Query(`
            SELECT DISTINCT tag
            FROM tags
            ORDER BY tag
        `)
	} else {
		rows, err = db.Query(`
            SELECT DISTINCT tag
            FROM tags
            WHERE tag LIKE ?
            ORDER BY tag
        `, "%"+tagFilter+"%")
	}

	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		err = rows.Scan(&tag)
		if err != nil {
			log.Fatal(err)
		}
		tags = append(tags, tag)
	}

	outputItems := makeTagOutputItems(tags)
	output := map[string][]OutputItem{"items": outputItems}
	jsonOutput, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(jsonOutput))
}

func makeTagOutputItems(tags []string) []OutputItem {
	rand.Seed(time.Now().UnixNano())
	outputItems := make([]OutputItem, len(tags))
	logseqGraph := os.Getenv("LogseqGraph")
	if logseqGraph == "" {
		logseqGraph = "Logseq"
	}

	for i, tag := range tags {
		outputItems[i] = OutputItem{
			UID:      fmt.Sprintf("%d", rand.Int()), // Generate random UID
			Title:    tag,
			Subtitle: "",
			Arg:      tag,
			Icon:     "icon.png",
		}
	}
	return outputItems
}
