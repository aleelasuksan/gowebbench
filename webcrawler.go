package main

import "fmt"
import "net/http"
import "crypto/tls"
import "net/url"
import "golang.org/x/net/html"
import "io"
import "flag"
import "os"
import "strconv"
import "regexp"
import "os/exec"

type CrawlInfo struct {
  URI string
  Depth int
}

var visited = make(map[string]bool)

func main() {
  flag.Parse()
  args := flag.Args()
  fmt.Println(args)
  if len(args) < 1 {
    fmt.Println("No URI given for crawling.")
    os.Exit(1)
  } else if len(args) < 2 {
    fmt.Println("No depth given for crawling.")
    os.Exit(1)
  }

  uri := args[0]
  depth, err := strconv.Atoi(args[1])

  if err != nil {
    fmt.Println("Second argument is not a number.")
    panic(err)
    os.Exit(1)
  }

  //var wg sync.WaitGroup
  //queue := make(chan CrawlInfo)
  //in := CrawlInfo{uri, depth}
  //go func() { queue <- in }()

  //for uri := range queue {
    fetchURI(uri, depth)
    //wg.Add(1)
    //go fetchURI(uri, queue, wg)
  //}
  fmt.Println("Done!")
  usr := 10
  trans := 5
  path := "D:\\src\\loadtest.go"
  for key, _ := range visited {
    fmt.Println()
    cmd := exec.Command("cmd", fmt.Sprintf("/C go run %s %s %d %d", path, key, usr, trans))
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Run()
    // output, err := cmd.Output()
    // if err != nil {
    //   fmt.Println("Exec commandline")
    //   panic(err)
    //   return
    // }
    // if len(output) > 0 {
    //   fmt.Println(string(output))
    // }
  }
}

func fetchURI(uri string, depth int) {
  if depth == 0 || visited[uri] {
    return
  }
  fmt.Println("fetching: ", uri , depth)
  visited[uri] = true
  transport := &http.Transport{
    TLSClientConfig: &tls.Config{
      InsecureSkipVerify: true,
    },
  }
  client := http.Client{Transport: transport}
  res, err := client.Get(uri)
  if err != nil {
    fmt.Println("Panic Get")
    panic(err)
    return
  }
  defer res.Body.Close()
  // fmt.Println(res.Header["Content-Type"])
  // fmt.Println(res.Header["Content-Length"])
  r, _ := regexp.Compile("html")
  if !r.MatchString(res.Header["Content-Type"][0]) {
    fmt.Println("Content-Type is not html.")
    return
  }
  links := fetchHyperLink(res.Body)
  for _, link := range links {
    absolutePath := normalizeURL(link, uri)
    if uri != "" {
      if !visited[absolutePath] {
        fetchURI(absolutePath, depth-1)
        //go func() { queue <- CrawlInfo{absolutePath, info.Depth} }()
      }
    }
  }
}

func fetchHyperLink(httpBody io.Reader) []string {
  links := make ([]string, 0)
  body := html.NewTokenizer(httpBody)
  for {
    tokenType := body.Next()
    if tokenType == html.ErrorToken {
      return links
    }
    token := body.Token()
    if tokenType == html.StartTagToken && token.DataAtom.String() == "a" {
      for _, attribute := range token.Attr {
        if attribute.Key == "href" {
          links = append(links, attribute.Val)
        }
      }
    }
  }
}

func normalizeURL(href, base string) string {
  uri, err := url.Parse(href)
  if err != nil {
    panic(err)
    return ""
  }
  baseURL, err := url.Parse(base)
  if err != nil {
    panic(err)
    return ""
  }
  uri = baseURL.ResolveReference(uri)
  return uri.String()
}
