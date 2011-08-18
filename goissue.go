package main

import (
	"bytes"
	"exec"
	"flag"
	"fmt"
	"html"
	"http"
	"io"
	"io/ioutil"
	"json"
	"log"
	"os"
	"path/filepath"
	"rand"
	"strings"
	"syscall"
	"url"
	"xml"
)

const version = "0.01"

var project = "go"

var xmlSpecial = map[byte]string{
	'<':  "&lt;",
	'>':  "&gt;",
	'"':  "&quot;",
	'\'': "&apos;",
	'&':  "&amp;",
}

type Link struct {
	Href     string "attr"
	Rel      string "attr"
	Type     string "attr"
	HrefLang string "attr"
}
type Author struct {
	Name  string "name"
	Uri   string "uri"
	Email string "email"
}
type IssuesCc struct {
	IssuesUri      string "issues:uri"
	IssuesUsername string "issues:username"
}
type IssuesOwner struct {
	IssuesUri      string "issues:uri"
	IssuesUsername string "issues:username"
}
type Entry struct {
	XMLNs         string        "attr"
	Id            string        "id"
	Published     string        "published"
	Updated       string        "updated"
	Title         string        "title"
	Content       string        "content"
	Link          []Link        "link"
	Author        []Author      "author"
	IssuesCc      []IssuesCc    "issues:cc"
	IssuesLabel   []string      "issues:label"
	IssuesOwner   []IssuesOwner "issues:owner"
	IssuesStars   []int         "issues:stars"
	IssuesState   []string      "issues:state"
	IssuesStatus  []string      "issues:status"
	IssuesSummary string        "issues:summary"
}

type Feed struct {
	Entry []Entry "entry"
}

// authLogin return auth code from AuthSub server.
// see: http://code.google.com/apis/accounts/docs/AuthForWebApps.html
func authLogin(config map[string]string) (auth string) {
	res, err := http.PostForm(
		"https://www.google.com/accounts/ClientLogin",
		url.Values(map[string][]string{
			"accountType": []string{"GOOGLE"},
			"Email":       []string{config["email"]},
			"Passwd":      []string{config["password"]},
			"service":     []string{"code"},
			"source":      []string{"golang-goissue-" + version},
		}))
	if err != nil {
		log.Fatal("failed to authenticate:", err)
	}
	defer res.Body.Close()
	b, _ := ioutil.ReadAll(res.Body)
	if res.StatusCode != 200 {
		log.Fatal("failed to authenticate:", res.Status)
	}
	lines := strings.Split(string(b), "\n")
	return lines[2]
}

// getConfig return string map of configuration that store email and password.
func getConfig() (config map[string]string) {
	file := ""
	if syscall.OS == "windows" {
		file = filepath.Join(os.Getenv("USERPROFILE"), "Application Data", "goissue", "settings.json")
	} else {
		file = filepath.Join(os.Getenv("HOME"), ".config", "goissue", "settings.json")
	}

	b, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatal("failed to read file "+file+":", err)
	}
	err = json.Unmarshal(b, &config)
	if err != nil {
		log.Fatal("failed to unmarhal settings.json:", err)
	}

	if _, ok := config["email"]; !ok {
		log.Fatal("failed to get email from your settings.json:", err)
	}
	if _, ok := config["password"]; !ok {
		log.Fatal("failed to get email from your settings.json:", err)
	}
	if _, ok := config["project"]; ok {
		project = config["project"]
	}
	return config
}

func dumpLevel(w io.Writer, n *html.Node, level int) os.Error {
	for i := 0; i < level; i++ {
		io.WriteString(w, "  ")
	}
	switch n.Type {
	case html.ErrorNode:
		return os.NewError("unexpected ErrorNode")
	case html.DocumentNode:
		return os.NewError("unexpected DocumentNode")
	case html.ElementNode:
	case html.TextNode:
		fmt.Fprintf(w, n.Data)
	case html.CommentNode:
		return os.NewError("COMMENT")
	default:
		return os.NewError("unknown node type")
	}
	for _, c := range n.Child {
		if err := dumpLevel(w, c, level+1); err != nil {
			return err
		}
	}
	return nil
}

func dump(n *html.Node) (string, os.Error) {
	if n == nil || len(n.Child) == 0 {
		return "", nil
	}
	b := bytes.NewBuffer(nil)
	for _, child := range n.Child {
		if err := dumpLevel(b, child, 0); err != nil {
			return "", err
		}
	}
	return b.String(), nil
}

// showIssue print issue detail.
func showIssue(auth string, id string) {
	req, err := http.NewRequest("GET", "https://code.google.com/feeds/issues/p/"+project+"/issues/full/"+id, nil)
	if err != nil {
		log.Fatal("failed to get issue:", err)
	}
	req.Header.Set("Authorization", "GoogleLogin "+auth)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal("failed to get issue:", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatal("failed to authenticate:", res.Status)
	}
	var entry Entry
	err = xml.Unmarshal(res.Body, &entry)
	if err != nil {
		log.Fatal("failed to get issue:", err)
	}
	doc, err := html.Parse(strings.NewReader(entry.Content))
	if err != nil {
		log.Fatal("failed to parse xml:", err)
	}
	text, err := dump(doc)
	if err != nil {
		log.Fatal("failed to parse xml:", err)
	}
	fmt.Println(entry.Title, "\n", text)
}

// searchIssues search word in issue list.
func searchIssues(auth, word string) {
	req, err := http.NewRequest("GET", "https://code.google.com/feeds/issues/p/"+project+"/issues/full?q="+url.QueryEscape(word), nil)
	if err != nil {
		log.Fatal("failed to get issues:", err)
	}
	req.Header.Set("Authorization", "GoogleLogin "+auth)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal("failed to get issues:", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatal("failed to get issues:", res.Status)
	}
	var feed Feed
	err = xml.Unmarshal(res.Body, &feed)
	if err != nil {
		log.Fatal("failed to parse xml:", err)
	}
	for _, entry := range feed.Entry {
		fmt.Println(entry.Id + ": " + entry.Title)
	}
}

// showIssues print issue list.
func showIssues(auth string) {
	req, err := http.NewRequest("GET", "https://code.google.com/feeds/issues/p/"+project+"/issues/full", nil)
	if err != nil {
		log.Fatal("failed to get issues:", err)
	}
	req.Header.Set("Authorization", "GoogleLogin "+auth)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal("failed to get issues:", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatal("failed to get issues:", res.Status)
	}
	var feed Feed
	err = xml.Unmarshal(res.Body, &feed)
	if err != nil {
		log.Fatal("failed to parse xml:", err)
	}
	for _, entry := range feed.Entry {
		fmt.Println(entry.Id + ": " + entry.Title)
	}
}

// showComments print comment list.
func showComments(auth string, id string) {
	req, err := http.NewRequest("GET", "https://code.google.com/feeds/issues/p/"+project+"/issues/"+id+"/comments/full", nil)
	if err != nil {
		log.Fatal("failed to get comments:", err)
	}
	req.Header.Set("Authorization", "GoogleLogin "+auth)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal("failed to get comments:", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatal("failed to authenticate:", res.Status)
	}
	var feed Feed
	err = xml.Unmarshal(res.Body, &feed)
	if err != nil {
		log.Fatal("failed to get comments:", err)
	}
	for _, entry := range feed.Entry {
		doc, err := html.Parse(strings.NewReader(entry.Content))
		if err != nil {
			log.Fatal("failed to parse xml:", err)
		}
		text, err := dump(doc)
		if err != nil {
			log.Fatal("failed to parse xml:", err)
		}
		fmt.Println(entry.Title, "\n", text)
	}
}

func run(argv []string) os.Error {
	cmd, err := exec.LookPath(argv[0])
	if err != nil {
		return err
	}
	var stdin *os.File
	if syscall.OS == "windows" {
		stdin, _ = os.Open("CONIN$")
	} else {
		stdin = os.Stdin
	}
	p, err := os.StartProcess(cmd, argv, &os.ProcAttr{Files: []*os.File{stdin, os.Stdout, os.Stderr}})
	if err != nil {
		return err
	}
	defer p.Release()
	w, err := p.Wait(0)
	if err != nil {
		return err
	}
	if !w.Exited() || w.ExitStatus() != 0 {
		return os.NewError("failed to execute text editor")
	}
	return nil
}

func xmlEscape(s string) string {
	var b bytes.Buffer
	for i := 0; i < len(s); i++ {
		c := s[i]
		if s, ok := xmlSpecial[c]; ok {
			b.WriteString(s)
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

func createIssue(auth string) {
	file := ""
	newf := fmt.Sprintf("%d.txt", rand.Int())
	if syscall.OS == "windows" {
		file = filepath.Join(os.Getenv("USERPROFILE"), "Application Data", "goissue", newf)
	} else {
		file = filepath.Join(os.Getenv("HOME"), ".config", "goissue", newf)
	}
	defer os.Remove(file)
	editor := os.Getenv("EDITOR")
	if len(editor) == 0 {
		if syscall.OS == "windows" {
			editor = "notepad"
		} else {
			editor = "vim"
		}
	}
	contents := `from: 
title: 
--------------
Before filing a bug, please check whether it has been fixed since
the latest release: run "hg pull -u" and retry what you did to
reproduce the problem.  Thanks.

What steps will reproduce the problem?
1.
2.
3.

What is the expected output?


What do you see instead?


Which compiler are you using (5g, 6g, 8g, gccgo)?


Which operating system are you using?


Which revision are you using?  (hg identify)


Please provide any additional information below.
`
	if syscall.OS == "windows" {
		contents = strings.Replace(contents, "\n", "\r\n", -1)
	}
	ioutil.WriteFile(file, []byte(contents), 0600)

	if err := run([]string{editor, file}); err != nil {
		log.Fatal("failed to create issue:", err)
	}

	b, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatal("failed to create issue:", err)
	}
	text := string(b)
	if syscall.OS == "windows" {
		text = strings.Replace(text, "\r\n", "\n", -1)
	}
	lines := strings.Split(text, "\n")
	if len(lines) < 4 {
		log.Fatal("failed to create issue")
	}
	from := lines[0]
	if len(from) < 7 || from[:6] != "from: " {
		log.Fatal("failed to create issue")
	}
	from = from[6:]
	title := lines[1]
	if len(title) < 8 || title[:7] != "title: " {
		log.Fatal("failed to create issue")
	}
	title = title[7:]
	body := strings.Join(lines[3:], "\n")

	/*
	entry := Entry{XMLNs: "http://www.w3.org/2005/Atom", Title: title, Content: body, Author: []Author{Author{Name: from}}, IssuesSummary: title}
	buf := bytes.NewBuffer(nil)
	err = xml.Marshal(buf, entry)
	if err != nil {
		log.Fatal("failed to post issue:", err)
	}
	str := "<?xml version='1.0' encoding='UTF-8'?>\n" + buf.String()
	str = strings.Replace(str, "<???", "<entry", 1)
	str = strings.Replace(str, "</???>", "</entry>", -1)
	*/
	str := fmt.Sprintf("<?xml version='1.0' encoding='UTF-8'?>\n"+
		"<entry xmlns='http://www.w3.org/2005/Atom' xmlns:issues='http://schemas.google.com/projecthosting/issues/2009'>\n"+
		"<title>%s</title>\n"+
		"<content type='html'>%s</content>\n"+
		"<author><name>%s</name></author>\n"+
		"<issues:updates>\n"+
		"<issues:summary>%s</issues:summary>\n"+
		"<issues:status>Started</issues:status>\n"+
		"<issues:label>-Type-Defect</issues:label>\n"+
		"<issues:label>-Priority-Medium</issues:label>\n"+
		"</issues:updates>\n"+
		"</entry>",
		xmlEscape(title),
		xmlEscape(body),
		xmlEscape(from),
		xmlEscape(title))
	req, err := http.NewRequest("POST", "https://code.google.com/feeds/issues/p/"+project+"/issues/full", strings.NewReader(str))
	if err != nil {
		log.Fatal("failed to post issue:", err)
	}
	req.Header.Set("Authorization", "GoogleLogin "+auth)
	req.Header.Set("Content-Type", "application/atom+xml")
	req.ContentLength = int64(len([]byte(str)))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal("failed to get issue:", err)
	}
	defer res.Body.Close()
	fmt.Println(res.Status)
}

func main() {
	search := flag.String("s", "", "search issues")
	create := flag.Bool("C", false, "create issue")
	comment := flag.Bool("c", false, "show comments")
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: goissue [-c ID | -s WORD]\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() > 1 {
		flag.Usage()
		os.Exit(1)
	}

	config := getConfig()
	auth := authLogin(config)

	if *create {
		createIssue(auth)
	} else if len(*search) > 0 {
		searchIssues(auth, *search)
	} else if flag.NArg() == 0 {
		showIssues(auth)
	} else {
		for i := 0; i < flag.NArg(); i++ {
			showIssue(auth, flag.Arg(i))
			if *comment {
				showComments(auth, flag.Arg(i))
			}
		}
	}
}
