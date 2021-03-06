package main

import (
    "flag"
    "bufio"
    "net"
    "fmt"
    "os"
    "io"
    "time"
    "bytes"
    "math/rand"
    "github.com/cespare/xxhash"
    "encoding/hex"
    "strconv"
    "github.com/gosuri/uiprogress"
    "log"
    "runtime"
    "runtime/pprof"
)

func main() {
    port := flag.Int("port", 4000, "Port to connect to.")
    connections := flag.Int("connections", 1000, "Amount of connections to make")
    parallel := flag.Int("parallel", 30, "Amount of parallel workers to run")
    cpuprofile := flag.String("cpuprofile", "", "write cpu profile to file")
    hide_progress := flag.Bool("hideProgress", false, "do not show progress bars")
    maxDelays := flag.Int("maxDelays", 0, "Randomly add a delay of max x seconds")
    flag.Parse()
    if *cpuprofile != "" {
        f, err := os.Create(*cpuprofile)
        if err != nil {
            log.Fatal(err)
        }
        pprof.StartCPUProfile(f)
        defer pprof.StopCPUProfile()
    }

    runtime.GOMAXPROCS(runtime.NumCPU() / 2)
    rand.Seed(time.Now().UnixNano())


    if !*hide_progress {
        uiprogress.Start()
    }


    done := make(chan bool)
    for i := 0; i < *parallel; i++ {
        prog := func () {}
        if !*hide_progress {
            my_prog := uiprogress.AddBar(*connections).AppendCompleted().PrependElapsed()
            prog = func() { my_prog.Incr() }
        }
        go start_bashing(rand.Float32() >= 0.5, *port, *connections, *maxDelays, prog, done)
    }

    result := true
    for i := 0; i < *parallel; i++ {
        result = result && <-done
    }
    if !*hide_progress {
        uiprogress.Stop()
    }

    if result {
        fmt.Println("OK")
        return
    }
    fmt.Println("ERROR")
    os.Exit(1)
}

const MAX_SIZE = 512 * 1024

func start_bashing(smallOnly bool, port int, connections int, delay int, progress func(), done chan bool) {
    max_request_size := MAX_SIZE
    if smallOnly {
        max_request_size = 128
    }
    for i := 0; i < connections; i++ {
        conn, err := net.Dial("tcp", ":" + strconv.Itoa(port))
        if err != nil {
            fmt.Println("ERROR", err)
            os.Exit(1)
        }

        if delay > 0 {
            time.Sleep(time.Duration(rand.Int31n(int32(delay))) * time.Second)
        }

        s := bufio.NewWriter(conn)

        wanted_size := rand.Int63n(int64(max_request_size))
        _, err = s.WriteString(strconv.Itoa(int(wanted_size)) + "\n")
        if err != nil {
            fmt.Println("ERROR", err)
            os.Exit(1)
        }
        s.Flush()

        if delay > 0 {
            time.Sleep(time.Duration(rand.Int31n(int32(delay))) * time.Second)
        }

        hasher := xxhash.New()
        _, err = io.CopyN(hasher, conn, wanted_size)
        if err != nil {
            fmt.Println("ERROR", err)
            os.Exit(1)
        }

        if delay > 0 {
            time.Sleep(time.Duration(rand.Int31n(int32(delay))) * time.Second)
        }

        _, err = s.WriteString(hex.EncodeToString(hasher.Sum(nil)) + "\n")
        if err != nil {
            fmt.Println("ERROR", err)
            os.Exit(1)
        }
        s.Flush()

        if delay > 0 {
            time.Sleep(time.Duration(rand.Int31n(int32(delay))) * time.Second)
        }

        result := make([]byte, 4)
        _, err = io.ReadFull(conn, result)
        if err != nil {
            fmt.Println("ERROR", err)
            os.Exit(1)
        }

        _, err = conn.Read(make([]byte, 1))
        if err == nil {
            println("Expected connection to be closed by now")
            os.Exit(1)
        }
        conn.Close()

        if !bytes.Equal(result, []byte{'O', 'K', 'O', 'K'}) {
            done <- false
            return
        }
        progress()
    }
    done <- true
}
