package main

import "fmt"
import "net/http"
import "crypto/tls"
import "net/url"
import "golang.org/x/net/html"
import "io"
import "flag"
import "os"
import "regexp"
import "log"
import "os/exec"
import "sync"
import "bytes"
import "runtime"

var visited = make(map[string]bool)

var wg sync.WaitGroup

var f *os.File

func main() {
  uriPtr := flag.String("uri", "http://www.google.com/", "uri to start crawling")
  depthPtr := flag.Int("depth", 1, "depth to crawl")
  loadPtr := flag.Bool("load", false, "do load testing")
  userPtr := flag.Int("user", 100, "number of concurrent users")
  transPtr := flag.Int("trans", 1, "number of transaction for each user")
  flag.Parse()

  runtime.GOMAXPROCS(runtime.NumCPU())

  uri := *uriPtr
  depth := *depthPtr
  load := *loadPtr

  if depth < 1 {
    fmt.Println("Depth is less than 1, Please specify depth equals 1 or greater.")
    os.Exit(1)
  }

  filename := "D:\\src\\crawling.log"
  var err error
  f, err = os.OpenFile(filename, os.O_WRONLY | os.O_APPEND | os.O_CREATE, 0666)
  if err != nil {
    log.Printf("%T %+v\n", err, err)
  }

  wg.Add(1)
  go fetchURI(uri, depth)

  wg.Wait()
  writeLog("Done Crawling!\r\n\r\n")
  fmt.Println("Done Crawling!\n")
  f.Close()
  if load {
      fmt.Println("Start Load Testing...")

      usr := *userPtr
      trans := *transPtr

      path := "D:\\src\\WTesting\\loadtest.go"
      for key, _ := range visited {
        fmt.Println()
        cmd := exec.Command("cmd", fmt.Sprintf("/C go run %s %s %d %d", path, key, usr, trans))
        cmd.Stdout = os.Stdout
        cmd.Stderr = os.Stderr
        cmd.Run()
      }
      fmt.Println("Done Load Testing!")
  }
}

func fetchURI(uri string, depth int) {
  fmt.Println("fetching: ", uri, depth)
  writeLog(fmt.Sprintf("fetching: %v %v\r\n", uri, depth))
  f.Sync()
  if visited[uri] {
    wg.Done()
    return
  } else if depth == 1 {
    visited[uri] = true
    wg.Done()
    return
  }
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
    writeLog("Panic Get")
    log.Printf("%T %+v\n", err, err)
    writeLog(fmt.Sprintf("%T %+v\r\n", err, err))
    return
  }
  defer res.Body.Close()
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
        wg.Add(1)
        go fetchURI(absolutePath, depth-1)
      }
    }
  }
  wg.Done()
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

func writeLog(message string) {
  var b bytes.Buffer
  _, err := fmt.Fprintf(&b, message)
  if err != nil {
    return
  }
  _, err = f.Write(b.Bytes())
  if err != nil {
    return
  }
}
