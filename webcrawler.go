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
import "log"
import "os/exec"
import "sync"
import "bytes"

var visited = make(map[string]bool)

var wg sync.WaitGroup

var f *os.File

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
    log.Printf("%T %+v\n", err, err)
    os.Exit(1)
  }

  if depth < 1 {
    fmt.Println("Depth is less than 1, Please specify depth equals 1 or greater.")
    os.Exit(1)
  }

  filename := "D:\\src\\crawling.log"
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
  if len(args) > 2 {
    if args[2] == "-load" {
      if len(args) < 4 {
        fmt.Println("No amount of concurrent user given.")
        os.Exit(1)
      } else if len(args) < 5 {
        fmt.Println("No amount of transaction per user given.")
        os.Exit(1)
      }

      fmt.Println("Start Load Testing...")

      usr, err := strconv.Atoi(args[3])
      if err != nil {
        fmt.Println("Fourth argument is not a number.")
        log.Printf("%T %+v\n", err, err)
        os.Exit(1)
      }
      trans, err := strconv.Atoi(args[4])
      if err != nil {
        fmt.Println("Fifth argument is not a number.")
        log.Printf("%T %+v\n", err, err)
        os.Exit(1)
      }

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
