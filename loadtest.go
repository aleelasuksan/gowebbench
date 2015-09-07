package main

import "fmt"
import "flag"
import "os"
import "net/http"
import "strconv"
import "time"
// import "runtime/debug"
import "runtime"
import "io/ioutil"
// import "sync"
import "log"
// import "net"
import "regexp"

type Response_Stat struct {
  status string
  response_time time.Duration
  amount_of_data int
}

func main() {
  flag.Parse()
  args := flag.Args()
  if len(args) < 1 {
    fmt.Println("Please specify uri")
    os.Exit(1)
  } else if len(args) < 2 {
    fmt.Println("Please specify number of concurrent users")
    os.Exit(1)
  } else if len(args) < 3 {
    fmt.Println("Please specify amount of transactions")
  }

  // debug.SetGCPercent(200)
  runtime.GOMAXPROCS(runtime.NumCPU())
  // runtime.GOMAXPROCS(1)
  uri := args[0]
  user, err := strconv.Atoi(args[1])
  if err != nil {
    log.Println(err)
    os.Exit(1)
  }
  trans, err := strconv.Atoi(args[2])
  if err != nil {
    log.Println(err)
    os.Exit(1)
  }

  // var wg sync.WaitGroup
  // wg.Add(user)

  result := make(chan Response_Stat, user)
  transport := http.Transport{
    DisableKeepAlives: false,
    MaxIdleConnsPerHost: user,
    // ResponseHeaderTimeout: 60 * time.Second,
    }

  start := time.Now()
  for i := 0 ; i < user ; i++ {
    go sendRequest(uri, trans, result, &transport)
    // time.Sleep(100 * time.Millisecond)
  }
  fmt.Printf("Start test %s...\n", uri)
  // wg.Wait()

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
      fmt.Printf("%4d : Status:%s ,Response time:%.4fsec ,Bytes:%v\n", count, s.status, s.response_time.Seconds(), s.amount_of_data)
      r, err := regexp.Compile("100|101|102|200|201|202|203|204|205|206|207|208|226|300|301|302|303|304|305|306|307|308")
      if err != nil {
        log.Printf("%T %+v\n", err, err)
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
    // case <-time.After(5 * time.Second):
    //   fmt.Println("TIMEOUT!")
    //   timeout = true
    }
  }
  end := time.Now()
  close(result)
  fmt.Println("=============== SUMMARY ================")
  fmt.Println("Target address:", uri)
  fmt.Println("Concurrent users:", user)
  fmt.Println("Total transaction:", user * trans)
  fmt.Println("Elapsed time:", end.Sub( start ))
  fmt.Println("Successful transaction:", success)
  fmt.Println("Failed transaction:", ( user * trans ) - success)
  fmt.Println("Total response data:", total_data)
  fmt.Println("Transaction rate:", float64( count ) / end.Sub( start ).Seconds(), "trans/sec")
  fmt.Println("Maximum response time:", max_res, "s")
  fmt.Println("Minimum response time:", min_res, "s")
  fmt.Println("Average response time:", sum_res / float64(success), "s")
}

func sendRequest(uri string, n int, result chan Response_Stat, transport *http.Transport) {
  for i := 0 ; i < n ; i++ {
    req, err := http.NewRequest( "GET", uri, nil )
    if err != nil {
      fmt.Println("Panic request")
      log.Printf("%T %+v\n", err, err)
      result <- Response_Stat{ "Error from Request", 0, 0 }
    } else {
      // req.Close = false
      start := time.Now()
      res, err := transport.RoundTrip(req)
      end := time.Now()
      response_time := end.Sub(start)
      if err != nil {
        fmt.Println("Panic response")
        log.Printf("%T %+v\n", err, err)
        result <- Response_Stat{ "Error from Response", response_time, 0 }
      } else {
        //result <- fmt.Sprintf("%s %v", res.Status, response_time)
        l := int(res.ContentLength)
        result <- Response_Stat{res.Status, response_time, l}
        ioutil.ReadAll(res.Body)
        res.Body.Close()
      }
    }
  }
  // wg.Donw()
}
