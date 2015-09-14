package main

import "fmt"
import "flag"
import "os"
import "net/http"
import "time"
// import "runtime/debug"
import "runtime"
import "io/ioutil"
import "log"
import "regexp"
import "bytes"

type Response_Stat struct {
  status string
  response_time time.Duration
  amount_of_data int
}

var f *os.File

func main() {
  uriPtr := flag.String("uri", "http://www.google.com/", "target uri for testing")
  userPtr := flag.Int("user", 100, "number of concurrent user")
  transPtr := flag.Int("trans", 1, "number of transaction for user to do request")
  filePtr := flag.String("output", "load.log", "path or filename for text output file")
  flag.Parse()

  // debug.SetGCPercent(200)
  runtime.GOMAXPROCS(runtime.NumCPU())
  uri := *uriPtr
  user := *userPtr
  trans := *transPtr

  filename := *filePtr

  var err error
  f, err = os.OpenFile(filename, os.O_WRONLY | os.O_APPEND | os.O_CREATE, 0666)
  if err != nil {
    log.Printf("%T %+v\n", err, err)
  }

  result := make(chan Response_Stat, user)
  transport := http.Transport{
    DisableKeepAlives: false,
    MaxIdleConnsPerHost: user,
    ResponseHeaderTimeout: 60 * time.Second,
    }

  start := time.Now()
  for i := 0 ; i < user ; i++ {
    go sendRequest(uri, trans, result, &transport)
  }

  fmt.Printf("Start test %s...\n", uri)
  writeLog(fmt.Sprintf("Start test %s...\r\n", uri))

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
  for ; count != user * trans && !timeout ; {
    select {
    case s := <-result:

      fmt.Printf("%6d : Status:%s ,Response time:%.4fsec ,Bytes:%v\n", count, s.status, s.response_time.Seconds(), s.amount_of_data)
      writeLog(fmt.Sprintf("%6d : Status:%s ,Response time:%.4fsec ,Bytes:%v\r\n", count, s.status, s.response_time.Seconds(), s.amount_of_data))

      r, err := regexp.Compile("100|101|102|200|201|202|203|204|205|206|207|208|226|300|301|302|303|304|305|306|307|308")
      if err != nil {
        log.Printf("%T %+v\n", err, err)
        writeLog(fmt.Sprintf("%T %+v\r\n", err, err))
      } else {
        if r.MatchString(s.status) == true {
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
  }
  end := time.Now()
  close(result)

  fmt.Println("=============== SUMMARY ================")
  writeLog("=============== SUMMARY ================\r\n")

  fmt.Println("Target address:", uri)
  writeLog(fmt.Sprintf("Target address: %v\r\n", uri))

  fmt.Println("Concurrent users:", user)
  writeLog(fmt.Sprintf("Concurrent users: %v \r\n", user))

  fmt.Println("Total transaction:", user * trans)
  writeLog(fmt.Sprintf("Total transaction: %v\r\n", user * trans))

  fmt.Println("Elapsed time:", end.Sub( start ))
  writeLog(fmt.Sprintf("Elapsed time: %v\r\n", end.Sub( start )))

  fmt.Println("Successful transaction:", success)
  writeLog(fmt.Sprintf("Successful transaction: %v\r\n", success))

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

  f.Close()
}

func sendRequest(uri string, n int, result chan Response_Stat, transport *http.Transport) {
  for i := 0 ; i < n ; i++ {
    req, err := http.NewRequest( "GET", uri, nil )
    if err != nil {
      fmt.Println("Panic request")
      writeLog("Panic request")
      log.Printf("%T %+v\n", err, err)
      writeLog(fmt.Sprintf("%T %+v\r\n", err, err))
      result <- Response_Stat{ "Error from Request", 0, 0 }
    } else {
      start := time.Now()
      res, err := transport.RoundTrip(req)
      end := time.Now()
      response_time := end.Sub(start)
      if err != nil {
        log.Printf("Panic Response %T %+v\n", err, err)
        writeLog(fmt.Sprintf("Panic Response %T %+v\r\n", err, err))
        result <- Response_Stat{ "Error from Response", response_time, 0 }
      } else {
        l := int(res.ContentLength)
        result <- Response_Stat{res.Status, response_time, l}
        ioutil.ReadAll(res.Body)
        res.Body.Close()
      }
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
