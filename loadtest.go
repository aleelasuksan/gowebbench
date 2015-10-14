package main

import "fmt"
import "flag"
import "os"
import "net/http"
import "time"
import "runtime"
import "io/ioutil"
import "log"
import "regexp"
import "bytes"
import "io"
import "bufio"
import "strings"
import "strconv"
import "net/url"

type Response_Stat struct {
  status string
  response_time time.Duration
  amount_of_data int
}

var f *os.File

func main() {
  uriPtr := flag.String("uri", "", "[Input mode] target uri for testing (use only one input mode)")
  userPtr := flag.Int("user", 1, "number of concurrent user")
  transPtr := flag.Int("trans", 1, "number of transaction for user to do request")
  filePtr := flag.String("output", "load.log", "path or filename for text output file")
  inputListPtr := flag.String("input", "", "[Input mode] path or filename for input file which use to read an address for load testing (use only one input mode)")
  flag.Parse()

  if *uriPtr == "" && *inputListPtr == "" {
    fmt.Println("Please specify target uri by using -uri=arg argument.")
    fmt.Println("Or specify input file path.")
    os.Exit(1)
  }

  if *uriPtr != "" && *inputListPtr != "" {
    fmt.Println("Both input mode specify.")
    fmt.Println("Use only one input mode. (-uri or -input flag)")
    os.Exit(1)
  }

  _, err := url.Parse(*uriPtr)
  if err != nil {
    log.Printf("%T %+v\n", err, err)
    os.Exit(1)
  }

  runtime.GOMAXPROCS(runtime.NumCPU())

  reader := bufio.NewReader(os.Stdin)
  fmt.Println("Execution might interrupt target server's function, Are you SURE?")
  fmt.Print("Confirm(y/n): ")
  in, _ := reader.ReadString('\n')
  in = strings.TrimSpace(in)
  if in == "y" || in == "Y" {
    fmt.Println("")
    load(*uriPtr, *userPtr, *transPtr, *inputListPtr, *filePtr)
  }
}

func load(uri string, user int, trans int, input string, filename string) {
  var err error
  if _, err = os.Stat(filename); err == nil {
    os.Remove(filename)
  }
  f, err = os.OpenFile(filename, os.O_WRONLY | os.O_CREATE, 0666)
  if err != nil {
    log.Printf("%T %+v\n", err, err)
  }
  defer f.Close()

  result := make(chan Response_Stat, user)
  defer close(result)

  transport := &http.Transport{
    DisableKeepAlives: false,
    MaxIdleConnsPerHost: user,
    ResponseHeaderTimeout: 60 * time.Second,
  }
  client := &http.Client{
    Transport: transport,
  }

  if input == "" {
    queueload(uri, user, trans, result, client)
  } else {
    infile, err := os.Open(input)
    if err != nil {
      log.Printf("%T %+v\n", err, err)
      return
    }
    defer infile.Close()
    r := bufio.NewReader(infile)
    err = nil
    var count int = 0
    reg, _ := regexp.Compile(".jpg|.png|.gif|.jpeg|.ico")
    start := time.Now()
    for err != io.EOF {
      var s string
      s, err = readLine(r)
      if err == nil && len(s) > 0 {
        arr := strings.Split(s, " ")
        if len(arr) == 1 {
          if reg.MatchString(s) {
            trans_reduced := trans/3
            if trans_reduced == 0 {
              trans_reduced = 1
            }
            queueload(arr[0], user, trans_reduced, result, client)
          } else {
            queueload(arr[0], user, trans, result, client)
          }
        } else {
          trans_reduced := trans
          depth, err := strconv.Atoi(arr[1])
          if err != nil {
            continue
          }
          for i := 0 ; i < depth ; i++ {
            trans_reduced = int(float64(trans_reduced)*0.8)
          }
          if trans_reduced == 0 {
            trans_reduced = 1
          }
          if reg.MatchString(s) {
            trans_reduced /= 3
            if trans_reduced == 0 {
              trans_reduced = 1
            }
            queueload(arr[0], user, trans_reduced, result, client)
          } else {
            queueload(arr[0], user, trans_reduced, result, client)
          }
        }

        fmt.Println()
        writeLog("\r\n")
        count++
      }
    }
    stop := time.Now()

    fmt.Printf("Total time: %v\n", stop.Sub(start))
    writeLog(fmt.Sprintf("Total time: %v\n", stop.Sub(start)))

    fmt.Printf("%v urls tested.\n", count)
    writeLog(fmt.Sprintf("%v urls tested.\n", count))

    fmt.Printf("%s DONE", time.Now())
    writeLog(fmt.Sprintf("%s DONE", time.Now().Format(time.RFC850)))
  }
}

func queueload(uri string, user int, trans int, result chan Response_Stat, client *http.Client) {
  start := time.Now()
  for i := 0 ; i < user ; i++ {
    go sendRequest(uri, trans, result, client)
  }

  fmt.Printf("%s Start test %s...\n", time.Now().Format(time.RFC850), uri)
  writeLog(fmt.Sprintf("%s Start test %s...\r\n", time.Now().Format(time.RFC850), uri))

  count := 0
  success := 0
  var min_res float64
  var max_res float64
  var sum_res float64
  min_res = 100
  max_res = 0
  sum_res = 0
  total_data := 0
  timeout := false

  r, err := regexp.Compile("^100|^101|^102|^200|^201|^202|^203|^204|^205|^206|^207|^208|^226|^300|^301|^302|^303|^304|^305|^306|^307|^308")
  if err != nil {
    log.Printf("%T %+v\n", err, err)
    writeLog(fmt.Sprintf("%T %+v\r\n", err, err))
  }

  for ; count != user * trans && !timeout ; {
    select {
    case s := <-result:

      fmt.Printf("%6d : Status:%s\n         Response time:%.4fsec ,Bytes:%v\n", count, s.status, s.response_time.Seconds(), s.amount_of_data)
      writeLog(fmt.Sprintf("%6d : Status:%s\r\n         Response time:%.4fsec ,Bytes:%v\r\n", count, s.status, s.response_time.Seconds(), s.amount_of_data))

      if r.MatchString(s.status) {
        if(s.response_time.Seconds() > max_res) {
          max_res = s.response_time.Seconds()
        }
        if(s.response_time.Seconds() < min_res) {
          min_res = s.response_time.Seconds()
        }
        sum_res += s.response_time.Seconds()
        if s.amount_of_data > 0 {
          total_data += s.amount_of_data
        }
        success++
      }
    }
    count++
  }
  end := time.Now()

  fmt.Println("=============== SUMMARY ================")
  writeLog("=============== SUMMARY ================\r\n")

  fmt.Printf("%s\n", time.Now().Format(time.RFC850))
  writeLog(fmt.Sprintf("%s\r\n", time.Now().Format(time.RFC850)))

  fmt.Println("Target address:", uri)
  writeLog(fmt.Sprintf("Target address: %v\r\n", uri))

  fmt.Println("Concurrent users:", user)
  writeLog(fmt.Sprintf("Concurrent users: %v \r\n", user))

  fmt.Println("Total transaction:", user * trans)
  writeLog(fmt.Sprintf("Total transaction: %v\r\n", user * trans))

  fmt.Println("Elapsed time:", end.Sub( start ))
  writeLog(fmt.Sprintf("Elapsed time: %v\r\n", end.Sub( start )))

  fmt.Println("Success transaction:", success)
  writeLog(fmt.Sprintf("Success transaction: %v\r\n", success))

  fmt.Println("Failed transaction:", ( user * trans ) - success)
  writeLog(fmt.Sprintf("Failed transaction: %v\r\n", ( user * trans ) - success))

  fmt.Println("Total response data:", total_data)
  writeLog(fmt.Sprintf("Total response data: %v \r\n", total_data))

  fmt.Println("Transaction rate:", float64( count ) / end.Sub( start ).Seconds(), "trans/sec")
  writeLog(fmt.Sprintf("Transaction rate: %v trans/sec\r\n", float64( count ) / end.Sub( start ).Seconds()))

  fmt.Println("Maximum response time:", max_res, "s")
  writeLog(fmt.Sprintf("Maximum response time: %v sec\r\n", max_res))

  fmt.Println("Minimum response time:", min_res, "s")
  writeLog(fmt.Sprintf("Minimum response time: %v sec\r\n", min_res))

  fmt.Println("Average response time:", sum_res / float64(success), "s")
  writeLog(fmt.Sprintf("Average response time: %v sec\r\n", sum_res / float64(success)))
}

func sendRequest(uri string, n int, result chan Response_Stat, client *http.Client) {
  for i := 0 ; i < n ; i++ {
    start := time.Now()
    res, err := client.Get(uri)
    end := time.Now()
    response_time := end.Sub(start)
    if err != nil {
      result <- Response_Stat{ fmt.Sprintf("Response Error%T %+v", err, err), response_time, 0 }
    } else {
      l := int(res.ContentLength)
      result <- Response_Stat{res.Status, response_time, l}
      ioutil.ReadAll(res.Body)
      res.Body.Close()
    }
  }
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

func readLine(reader *bufio.Reader) (string, error) {
  isPrefix := true
  var err error = nil
  var line, text []byte
  for isPrefix {
    line, isPrefix, err = reader.ReadLine()
    if err != io.EOF && err != nil {
      log.Printf("%T %+v\n", err, err)
    }
    text = append(text, line ...)
  }
  return string(text), err
}
