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
import "io/ioutil"

var visited = make(map[string]bool)

var wg sync.WaitGroup

var f *os.File

func main() {
  uriPtr := flag.String("uri", "http://www.google.com/", "uri to start crawling")
  depthPtr := flag.Int("depth", 1, "depth to crawl")
  loadPtr := flag.Bool("load", false, "do load testing")
  userPtr := flag.Int("user", 100, "number of concurrent users")
  transPtr := flag.Int("trans", 1, "number of transaction for each user")
  filePtr := flag.String("output", "crawl_result.log", "path or filename for text output file")
  flag.Parse()

  runtime.GOMAXPROCS(runtime.NumCPU())

  address := parseURIwithoutFragment(*uriPtr)
  if address == nil {
    fmt.Println("Given URI is invalid.")
    os.Exit(1)
  }
  base, _ := regexp.Compile(strings.Replace(address.Host, ".", "\\.", -1))
  uri := address.String()
  depth := *depthPtr
  load := *loadPtr
  r, _ := regexp.Compile("html")

  if depth < 0 {
    fmt.Println("Depth is less than 0, Please specify depth equals 0 or greater.")
    os.Exit(1)
  }

  filename := "crawling.log"
  var err error
  f, err = os.OpenFile(filename, os.O_WRONLY | os.O_APPEND | os.O_CREATE, 0666)
  if err != nil {
    log.Printf("%T %+v\n", err, err)
    os.Exit(1)
  }
  transport := &http.Transport{
    TLSClientConfig: &tls.Config{
      InsecureSkipVerify: true,
    },
  }
  client := &http.Client {
    Transport: transport,
  }
  visited[uri] = true

  wg.Add(1)
  go fetchURI(uri, depth, base, r, client)
  time.Sleep(1 * time.Second)
  wg.Wait()
  writeLog("Done Crawling!\r\n\r\n")
  fmt.Println("Done Crawling!\n")
  f.Close()

  filename = *filePtr
  // // os.O_APPEND to append result file
  f, err = os.OpenFile(filename, os.O_WRONLY | os.O_CREATE, 0666)
  if err != nil {
    log.Printf("%T %+v\n", err, err)
    os.Exit(1)
  }
  count := 0

  fmt.Printf("Write Result to file %v ...\n", filename)

  for key, value := range visited {
    if value {
      count++
      // fmt.Println(key)
      writeLog(fmt.Sprintf("%v\r\n", key))
    }
  }
  // writeLog(fmt.Sprintf("%v uri found.\r\n\r\n", count))
  fmt.Printf("%v uri found.\n", count)

  if load {
    loadtest(*userPtr, *transPtr, filename)
  }
}

func fetchURI(uri string, depth int, base *regexp.Regexp, reghtml *regexp.Regexp, client *http.Client) {

  defer wg.Done()
  // fmt.Println("fetching: ", uri, depth)
  // writeLog(fmt.Sprintf("fetching: %v %v %v\r\n", uri, depth))
  if depth == 0 {
    // req, err := http.NewRequest( "HEAD", uri, nil)
    // if err != nil {
    //   return
    // }
    // res, err := transport.RoundTrip(req)
    res, err := client.Head(uri)
    if err != nil {
      log.Printf("Panic Head %v %v\n%T %+v\n", uri, depth, err, err)
      writeLog(fmt.Sprintf("Panic Head %v %v\n%T %+v\r\n", uri, depth, err, err))
      return
    }

    defer res.Body.Close()

    fmt.Printf("fetched: %v %v\n", uri, depth)

    if !reghtml.MatchString(res.Header.Get("Content-Type")) {
      visited[uri] = false
    }
    return
  } else {
    // req, err := http.NewRequest( "GET", uri, nil )
    // if err != nil {
    //   return
    // }
    // res, err := transport.RoundTrip(req)
    res, err := client.Get(uri)
    if err != nil {
      log.Printf("Panic Get %v %v\n%T %+v\n", uri, depth, err, err)
      writeLog(fmt.Sprintf("Panic Get %v %v\r\n%T %+v\r\n", uri, depth, err, err))
      return
    }
    defer res.Body.Close()
    fmt.Printf("fetched: %v %v\n", uri, depth)

    if !reghtml.MatchString(res.Header.Get("Content-Type")) {
      visited[uri] = false
      return
    }
    links := fetchHyperLink(res.Body)
    for _, link := range links {
      absolutePath := normalizeURL(link, uri)
      if absolutePath != "" {

        address := parseURIwithoutFragment(absolutePath)

        // if request uri host/domain doesn't match base host then ignore this uri
        if address == nil || !base.MatchString(address.Host) {
          continue
        }

        target := address.String()
        if !visited[target] {
          visited[target] = true
          wg.Add(1)
          go fetchURI(target, depth-1, base, reghtml, client)
        }
      }
    }
  }
}

func loadtest(user int, trans int, filename string) {
  fmt.Println("Start Load Testing...")
  path := "loadtest.go"
  start := time.Now()

  fmt.Println()
  cmd := exec.Command("cmd", fmt.Sprintf("/C go run %s -input=%s -user=%d -trans=%d", path, filename, user, trans))
  cmd.Stdout = os.Stdout
  cmd.Stderr = os.Stderr
  cmd.Run()

  end := time.Now()
  fmt.Println("Done Load Testing!")
  fmt.Printf("Total time: %v\n", end.Sub(start))
  fmt.Printf("%v uri.\n", len(visited))
}

func fetchHyperLink(httpBody io.Reader) []string {
  defer ioutil.ReadAll(httpBody)
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
    return ""
  }
  baseURL, err := url.Parse(base)
  if err != nil {
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
    return nil
  }
  if len(address.Path) == 0 {
    address.Path = "/"
  }
  address.Fragment = ""
  address.RawQuery = ""
  return address
}
