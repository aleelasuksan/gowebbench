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
import "sync"
import "bytes"
import "runtime"
import "time"
import "strings"
import "io/ioutil"
import "math"

var visited = make(map[string]int)

var wg sync.WaitGroup

var f *os.File

var limit int

func main() {
  uriPtr := flag.String("uri", "", "uri to start crawling")
  depthPtr := flag.Int("depth", 1, "depth to crawl")
  filePtr := flag.String("output", "crawl_result.log", "path or filename for text output file")
  limitPtr := flag.Float64("limit", -1, "limit number of crawled urls. (less than zero mean no limitation)")
  flag.IntVar(depthPtr, "d", 1, "depth to crawl")
  flag.Parse()

  if *uriPtr == "" {
    fmt.Println("Please specify target uri by using -uri=arg argument.")
    os.Exit(1)
  }

  if *depthPtr < 0 {
    fmt.Println("Depth is less than 0, Please specify depth equals 0 or greater.")
    os.Exit(1)
  }

  runtime.GOMAXPROCS(runtime.NumCPU())

  crawl(*uriPtr, *depthPtr, *limitPtr, *filePtr)
}

func crawl(add string, depth int, lim float64, filename string ) {
  address := parseURIwithoutFragment(add)
  if address == nil {
    fmt.Println("Given URL is invalid.")
    os.Exit(1)
  }
  base, _ := regexp.Compile(strings.Replace(address.Host, ".", "\\.", -1))
  uri := address.String()
  limit = int(lim * ( 1 + ( math.Log10( lim ) / 100 ) ) )
  r, _ := regexp.Compile("htm|image|html|javascript|css|jpeg|jpg|png|gif|woff|ttf|ico")

  logfile := "crawling.log"
  var err error
  if _, err := os.Stat(logfile); err == nil {
    os.Remove(logfile)
  }
  f, err = os.OpenFile(logfile, os.O_WRONLY | os.O_CREATE, 0666)
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
  visited[uri] = 1

  fmt.Printf("%s Start crawling...\n", time.Now().Format(time.RFC850))
  writeLog(fmt.Sprintf("%s Start crawling...\r\n", time.Now().Format(time.RFC850)))

  wg.Add(1)
  go fetchURI(uri, depth, base, r, client)
  time.Sleep(1 * time.Second)
  wg.Wait()

  writeLog(fmt.Sprintf("%s Done Crawling!\r\n\r\n", time.Now().Format(time.RFC850)))
  fmt.Printf("%s Done Crawling!\n\n", time.Now().Format(time.RFC850))

  f.Close()

  if _, err := os.Stat(filename); err == nil {
    os.Remove(filename)
  }
  // os.O_APPEND to append result file
  f, err = os.OpenFile(filename, os.O_WRONLY | os.O_CREATE, 0666)
  if err != nil {
    log.Printf("%T %+v\n", err, err)
    os.Exit(1)
  }
  count := 0

  fmt.Printf("Write Result to file %v ...\n", filename)

  for key, value := range visited {
    count++
    writeLog(fmt.Sprintf("%v %v\r\n", key, depth-value))
  }
  fmt.Printf("%v uri found.\n", count)
}

func fetchURI(uri string, depth int, base *regexp.Regexp, reghtml *regexp.Regexp, client *http.Client) {
  defer wg.Done()

  if limit > 0 && len(visited) > limit {
    delete(visited, uri)
    return
  }

  if depth == 0 {
    res, err := client.Head(uri)
    if err != nil {
      log.Printf("Panic Head %v %v\n%T %+v\n", uri, depth, err, err)
      writeLog(fmt.Sprintf("Panic Head %v %v\n%T %+v\r\n", uri, depth, err, err))
      return
    }
    defer res.Body.Close()

    writeLog(fmt.Sprintf("Fetch: %v %v\r\n%v\r\n", uri, depth, res.Status))
    fmt.Printf("Fetched: %v %v\n%v\n", uri, depth, res.Status)

    if !reghtml.MatchString(res.Header.Get("Content-Type")) {
      delete(visited, uri)
    }
    return
  } else {
    res, err := client.Get(uri)
    if err != nil {
      log.Printf("Panic Get %v %v\n%T %+v\n", uri, depth, err, err)
      writeLog(fmt.Sprintf("Panic Get %v %v\r\n%T %+v\r\n", uri, depth, err, err))
      return
    }
    defer res.Body.Close()

    writeLog(fmt.Sprintf("Fetch: %v %v\r\n%v\r\n", uri, depth, res.Status))
    fmt.Printf("Fetched: %v %v\n%v\n", uri, depth, res.Status)

    if !reghtml.MatchString(res.Header.Get("Content-Type")) {
      delete(visited, uri)
      return
    }
    if !strings.Contains(res.Header.Get("Content-Type"), "html") {
      return
    }

    links := fetchHyperLink(res.Body)
    for _, link := range links {
      absolutePath := normalizeURL(link, uri)
      if absolutePath != "" {
        address := parseURIwithoutFragment(absolutePath)
        // if request uri host/domain doesn't match base host then ignore
        if address == nil || !base.MatchString(address.Host) {
          continue
        }
        target := address.String()
        target, err = url.QueryUnescape(target)
        if err != nil {
          continue
        }
        if visited[target] < 1 {
          visited[target] = depth-1
          wg.Add(1)
          go fetchURI(target, depth-1, base, reghtml, client)
        }
      }
    }
  }
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
    if tokenType == html.StartTagToken {
      if token.DataAtom.String() == "a" || token.DataAtom.String() == "link" {
        for _, attribute := range token.Attr {
          if attribute.Key == "href" {
            links = append(links, attribute.Val)
          }
        }
      } else if token.DataAtom.String() == "img" || token.DataAtom.String() == "script" {
        for _, attribute := range token.Attr {
          if attribute.Key == "src" {
            links = append(links, attribute.Val)
          }
        }
      }
    }
  }
  return links
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
