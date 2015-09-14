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
import "time"
import "strings"

var visited = make(map[string]bool)

var wg sync.WaitGroup

var f *os.File

var jobQueue chan bool

type Worker struct {
  WorkerPool  chan chan bool
}

func main() {
  uriPtr := flag.String("uri", "http://www.google.com/", "uri to start crawling")
  depthPtr := flag.Int("depth", 1, "depth to crawl")
  loadPtr := flag.Bool("load", false, "do load testing")
  userPtr := flag.Int("user", 100, "number of concurrent users")
  transPtr := flag.Int("trans", 1, "number of transaction for each user")
  filePtr := flag.String("output", "crawling.log", "path or filename for text output file")
  flag.Parse()

  runtime.GOMAXPROCS(runtime.NumCPU())

  address := parseURIwithoutFragment(*uriPtr)
  base, _ := regexp.Compile(strings.Replace(address.Host, ".", "\\.", -1))
  uri := address.String()
  depth := *depthPtr
  load := *loadPtr

  if depth < 1 {
    fmt.Println("Depth is less than 1, Please specify depth equals 1 or greater.")
    os.Exit(1)
  }

  filename := *filePtr
  var err error
  f, err = os.OpenFile(filename, os.O_WRONLY | os.O_APPEND | os.O_CREATE, 0666)
  if err != nil {
    log.Printf("%T %+v\n", err, err)
    os.Exit(1)
  }

  wg.Add(1)
  go fetchURI(uri, depth, base)
  time.Sleep(1 * time.Second)
  wg.Wait()
  writeLog("Done Crawling!\r\n\r\n")
  fmt.Println("Done Crawling!\n")
  f.Close()

  filename = "crawling_result.log"
  f, err = os.OpenFile(filename, os.O_WRONLY | os.O_APPEND | os.O_CREATE, 0666)
  if err != nil {
    log.Printf("%T %+v\n", err, err)
    os.Exit(1)
  }
  count := 0

  fmt.Println("Result")
  writeLog("Result\r\n")

  for key, value := range visited {
    if value {
      count++
      fmt.Printf("%4v: %v\n", count, key)
      writeLog(fmt.Sprintf("%4v: %v\r\n", count, key))
    }
  }
  writeLog(fmt.Sprintf("%v uri found.", count))
  fmt.Printf("%v uri found.\n", count)

  if load {
      fmt.Println("Start Load Testing...")

      usr := *userPtr
      trans := *transPtr
      path := "loadtest.go"
      start := time.Now()
      for key, value := range visited {
        if value {
          fmt.Println()
          cmd := exec.Command("cmd", fmt.Sprintf("/C go run %s -uri=%s -user=%d -trans=%d", path, key, usr, trans))
          cmd.Stdout = os.Stdout
          cmd.Stderr = os.Stderr
          cmd.Run()
        }
      }
      end := time.Now()
      fmt.Println("Done Load Testing!")
      fmt.Printf("Total time: %v\n", end.Sub(start))
      fmt.Printf("%v uri.\n", len(visited))
  }
}

func fetchURI(uri string, depth int, base *regexp.Regexp) {
  defer wg.Done()
  address := parseURIwithoutFragment(uri)

  // if request uri host/domain doesn't match base host then ignore this uri
  if address == nil || !base.MatchString(address.Host) {
    return
  }

  target := address.String()
  if visited[target] {
    return
  }

  fmt.Println("fetching: ", target, depth)
  writeLog(fmt.Sprintf("fetching: %v %v\r\n", target, depth))
  visited[target] = true
  transport := &http.Transport{
    TLSClientConfig: &tls.Config{
      InsecureSkipVerify: true,
    },
  }
  if depth == 1 {
    req, err := http.NewRequest( "HEAD", target, nil)
    if err != nil {
      log.Printf("%T %+v\n", err, err)
      writeLog(fmt.Sprintf("%T %+v\r\n", err, err))
      return
    }
    res, err := transport.RoundTrip(req)
    if err != nil {
      log.Printf("Panic Head %v %v\n%T %+v\n", target, depth, err, err)
      writeLog(fmt.Sprintf("Panic Head %v %v\r\n%T %+v\r\n", target, depth, err, err))
      return
    }

    defer res.Body.Close()

    fmt.Printf("fetched: %v %v %v\n",target, depth, res.Header.Get("Content-Type"))

    r, _ := regexp.Compile("html")
    if !r.MatchString(res.Header.Get("Content-Type")) {
      visited[target] = false
    }
    return
  }

  req, err := http.NewRequest( "GET", target, nil )
  if err != nil {
    log.Printf("%T %+v\n", err, err)
    writeLog(fmt.Sprintf("%T %+v\r\n", err, err))
    return
  }
  res, err := transport.RoundTrip(req)
  if err != nil {
    log.Printf("Panic Get %v %v\n%T %+v\n", target, depth, err, err)
    writeLog(fmt.Sprintf("Panic Get %v %v\r\n%T %+v\r\n", target, depth, err, err))
    return
  }
  defer res.Body.Close()
  fmt.Printf("fetched: %v %v %v\n", target, depth, res.Header.Get("Content-Type"))
  // following regexp check for content-type if it is not html then ignore to use in load testing
  r, _ := regexp.Compile("html")
  if !r.MatchString(res.Header.Get("Content-Type")) {
    visited[target] = false
    return
  }
  links := fetchHyperLink(res.Body)
  for _, link := range links {
    absolutePath := normalizeURL(link, target)
    if absolutePath != "" {
      if !visited[absolutePath] {
        wg.Add(1)
        go fetchURI(absolutePath, depth-1, base)
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
  if uri.Scheme != "http" && uri.Scheme != "https" {
    return ""
  }
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
  f.Sync()
}

func parseURIwithoutFragment(s string) *url.URL{
  address, err := url.Parse(s)
  if err != nil {
    log.Printf("%T %+v\n", err, err)
    writeLog(fmt.Sprintf("%T %+v\r\n", err, err))
    return nil
  }

  address.Fragment = ""
  return address
}
